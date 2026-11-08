package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fenet-as/KnowFlow/internal/chunk"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/embed"
	"github.com/fenet-as/KnowFlow/internal/platform"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

type SeedOptions struct {
	CorpusDir string
	DBURL     string
	OwnerID   string
	Clear     bool
	APIURL    string
	APIToken  string
}

func main() {
	var opts SeedOptions

	flag.StringVar(&opts.CorpusDir, "dir", "demo/corpus", "Path to demo corpus directory")
	flag.StringVar(&opts.DBURL, "db", "", "PostgreSQL database URL (defaults to DATABASE_URL)")
	flag.StringVar(&opts.OwnerID, "owner", "user_demo_knowflow", "Owner ID for seeded documents")
	flag.BoolVar(&opts.Clear, "clear", true, "Clear existing documents for this owner before seeding")
	flag.StringVar(&opts.APIURL, "api", "", "Seed via HTTP API instead of direct DB (e.g. http://localhost:8080)")
	flag.StringVar(&opts.APIToken, "token", "", "Clerk JWT token if seeding via HTTP API")
	flag.Parse()

	if opts.DBURL == "" && opts.APIURL == "" {
		opts.DBURL = os.Getenv("DATABASE_URL")
		if opts.DBURL == "" {
			opts.DBURL = "postgres://knowflow:knowflow@localhost:5433/knowflow?sslmode=disable"
		}
	}

	fmt.Println("=================================================================")
	fmt.Println("             KnowFlow Demo Corpus Seeder (KF-41)                 ")
	fmt.Println("=================================================================")
	fmt.Printf("Corpus Directory:   %s\n", opts.CorpusDir)
	fmt.Printf("Target Owner ID:    %s\n", opts.OwnerID)
	fmt.Printf("Clear Existing:     %v\n", opts.Clear)
	if opts.APIURL != "" {
		fmt.Printf("Seed Mode:          HTTP API (%s)\n", opts.APIURL)
	} else {
		fmt.Printf("Seed Mode:          Direct Database (%s)\n", opts.DBURL)
	}
	fmt.Println("-----------------------------------------------------------------")

	// Read corpus documents
	files, err := os.ReadDir(opts.CorpusDir)
	if err != nil {
		fmt.Printf("Error reading corpus directory '%s': %v\n", opts.CorpusDir, err)
		os.Exit(1)
	}

	var mdFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			mdFiles = append(mdFiles, f.Name())
		}
	}

	if len(mdFiles) == 0 {
		fmt.Printf("No markdown files found in '%s'.\n", opts.CorpusDir)
		os.Exit(1)
	}

	fmt.Printf("Discovered %d demo corpus documents:\n", len(mdFiles))
	for _, name := range mdFiles {
		fmt.Printf("  • %s\n", name)
	}
	fmt.Println("-----------------------------------------------------------------")

	if opts.APIURL != "" {
		seedViaAPI(opts, mdFiles)
	} else {
		seedViaDatabase(opts, mdFiles)
	}
}

func seedViaDatabase(opts SeedOptions, mdFiles []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	poolCfg := platform.DefaultDBPoolConfig()
	pool, err := platform.NewDBPool(ctx, opts.DBURL, poolCfg)
	if err != nil {
		fmt.Printf("Failed to connect to database at '%s': %v\n", opts.DBURL, err)
		os.Exit(1)
	}
	defer pool.Close()

	docRepo := repo.NewPostgresDocumentRepo(pool)
	chunkRepo := repo.NewPostgresChunkRepo(pool)

	if opts.Clear {
		fmt.Printf("Clearing existing documents for owner '%s'...\n", opts.OwnerID)
		tag, err := pool.Exec(ctx, "DELETE FROM documents WHERE owner_id = $1", opts.OwnerID)
		if err != nil {
			fmt.Printf("Warning: failed to clear documents: %v\n", err)
		} else {
			fmt.Printf("Removed %d previous documents.\n", tag.RowsAffected())
		}
	}

	// Determine embedder: use real Gemini embedder if key is set, otherwise FakeEmbedder
	var embedder embed.Embedder
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey != "" && apiKey != "dev_key" {
		fmt.Println("Using Google Gemini Embedder (text-embedding-004)...")
		aiLimiter := rate.NewLimiter(rate.Limit(5), 10)
		geminiEmbedder, err := embed.NewGeminiEmbedder(embed.GeminiEmbedderConfig{
			APIKey:    apiKey,
			Model:     "text-embedding-004",
			Dimension: 768,
			Timeout:   15 * time.Second,
			Limiter:   aiLimiter,
		})
		if err != nil {
			fmt.Printf("Failed to initialize Gemini embedder: %v. Using FakeEmbedder.\n", err)
			embedder = embed.NewFakeEmbedder(768)
		} else {
			embedder = geminiEmbedder
		}
	} else {
		fmt.Println("Using FakeEmbedder (deterministic offline vectors)...")
		embedder = embed.NewFakeEmbedder(768)
	}

	chunker := chunk.NewMarkdownChunker(chunk.ChunkerOptions{
		TargetTokens: 650,
		MaxTokens:    800,
		OverlapPct:   12,
	})

	totalChunks := 0
	startTime := time.Now()

	for _, filename := range mdFiles {
		path := filepath.Join(opts.CorpusDir, filename)
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("Failed to read %s: %v\n", filename, err)
			continue
		}

		doc := &domain.Document{
			OwnerID:     opts.OwnerID,
			Filename:    filename,
			SizeBytes:   int64(len(content)),
			ContentHash: fmt.Sprintf("seed_%s_%d", filename, time.Now().Unix()),
			RawContent:  string(content),
			Status:      "ready",
		}

		job, err := docRepo.CreateWithJob(ctx, doc, 3)
		if err != nil {
			fmt.Printf("Failed to create document record for %s: %v\n", filename, err)
			continue
		}

		// Segment into semantic chunks
		chunks, err := chunker.Split(string(content))
		if err != nil {
			fmt.Printf("Chunker failed for %s: %v\n", filename, err)
			continue
		}

		if len(chunks) > 0 {
			texts := make([]string, len(chunks))
			for i, ch := range chunks {
				texts[i] = ch.Content
			}

			vecs, err := embedder.Embed(ctx, texts, embed.EmbedKindDocument)
			if err != nil {
				fmt.Printf("Embedding failed for %s: %v\n", filename, err)
				continue
			}

			domainChunks := make([]*domain.Chunk, len(chunks))
			for i, ch := range chunks {
				domainChunks[i] = &domain.Chunk{
					ID:          uuid.New(),
					DocumentID:  doc.ID,
					OwnerID:     opts.OwnerID,
					ChunkIndex:  i,
					Content:     ch.Content,
					Heading:     ch.Heading,
					TokenCount:  ch.TokenCount,
					Embedding:   vecs[i],
				}
			}

			if err := chunkRepo.BulkInsert(ctx, domainChunks); err != nil {
				fmt.Printf("Failed to insert chunks for %s: %v\n", filename, err)
				continue
			}

			// Update document to ready with chunk count
			_, _ = pool.Exec(ctx,
				"UPDATE documents SET status = 'ready', chunk_count = $1, updated_at = now() WHERE id = $2",
				len(domainChunks), doc.ID,
			)

			// Mark job succeeded
			_, _ = pool.Exec(ctx,
				"UPDATE jobs SET status = 'succeeded' WHERE id = $1",
				job.ID,
			)

			totalChunks += len(domainChunks)
			fmt.Printf("  ✓ %-35s -> %2d chunks ingested\n", filename, len(domainChunks))
		}
	}

	fmt.Println("-----------------------------------------------------------------")
	fmt.Printf("Seeding Complete! Successfully ingested %d documents (%d chunks) in %v.\n",
		len(mdFiles), totalChunks, time.Since(startTime).Round(time.Millisecond))
	fmt.Printf("Owner '%s' is ready for interactive demo.\n", opts.OwnerID)
	fmt.Println("=================================================================")
}

func seedViaAPI(opts SeedOptions, mdFiles []string) {
	client := &http.Client{Timeout: 30 * time.Second}
	uploadURL := strings.TrimSuffix(opts.APIURL, "/") + "/api/v1/documents"

	fmt.Printf("Uploading %d files via %s...\n", len(mdFiles), uploadURL)

	for _, filename := range mdFiles {
		path := filepath.Join(opts.CorpusDir, filename)
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("Failed to read %s: %v\n", filename, err)
			continue
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			fmt.Printf("Failed to create form file for %s: %v\n", filename, err)
			continue
		}
		_, _ = part.Write(content)
		_ = writer.Close()

		req, err := http.NewRequest(http.MethodPost, uploadURL, body)
		if err != nil {
			fmt.Printf("Failed to create HTTP request: %v\n", err)
			continue
		}

		req.Header.Set("Content-Type", writer.FormDataContentType())
		if opts.APIToken != "" {
			req.Header.Set("Authorization", "Bearer "+opts.APIToken)
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Upload request failed for %s: %v\n", filename, err)
			continue
		}

		respBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
			var result struct {
				Data struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"data"`
			}
			_ = json.Unmarshal(respBytes, &result)
			fmt.Printf("  ✓ Uploaded %-35s (ID: %s, Status: %s)\n", filename, result.Data.ID, result.Data.Status)
		} else {
			fmt.Printf("  ✗ Failed %-37s Status: %d Body: %s\n", filename, resp.StatusCode, string(respBytes))
		}
	}

	fmt.Println("-----------------------------------------------------------------")
	fmt.Println("API Uploads completed. Background worker pool is processing chunks.")
	fmt.Println("Check the KnowFlow UI document list to verify all documents reach 'ready' status.")
	fmt.Println("=================================================================")
}
