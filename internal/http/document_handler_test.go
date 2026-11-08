package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDocService struct {
	uploadFn  func(ctx context.Context, ownerID string, filename string, content []byte) (*domain.Document, error)
	getByIDFn func(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Document, error)
	listFn    func(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Document, *string, error)
	deleteFn  func(ctx context.Context, ownerID string, id uuid.UUID) error
}

func (m *mockDocService) Upload(ctx context.Context, ownerID string, filename string, content []byte) (*domain.Document, error) {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, ownerID, filename, content)
	}
	return &domain.Document{
		ID:        uuid.New(),
		OwnerID:   ownerID,
		Filename:  filename,
		SizeBytes: int64(len(content)),
		Status:    "queued",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (m *mockDocService) GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Document, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, ownerID, id)
	}
	return nil, domain.NewNotFoundError("Document not found.")
}

func (m *mockDocService) List(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Document, *string, error) {
	if m.listFn != nil {
		return m.listFn(ctx, ownerID, limit, cursor)
	}
	return nil, nil, nil
}

func (m *mockDocService) Delete(ctx context.Context, ownerID string, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, ownerID, id)
	}
	return nil
}

func setupTestDocumentRouter(svc *mockDocService, ownerID string, maxUploadBytes int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewDocumentHandler(svc, maxUploadBytes)

	// Auth simulator middleware
	authMiddleware := func(c *gin.Context) {
		if ownerID != "" {
			c.Request = c.Request.WithContext(auth.WithOwnerID(c.Request.Context(), ownerID))
		}
		c.Next()
	}

	api := r.Group("/api/v1")
	api.Use(authMiddleware)
	{
		api.POST("/documents", handler.Upload)
		api.GET("/documents", handler.List)
		api.GET("/documents/:id", handler.GetByID)
		api.DELETE("/documents/:id", handler.Delete)
	}

	return r
}

func createMultipartRequest(url, fieldName, fileName string, content []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestDocumentHandler_Upload(t *testing.T) {
	mockSvc := &mockDocService{}
	router := setupTestDocumentRouter(mockSvc, "user_clerk_test", 1024)

	t.Run("successful .txt upload returns 202 Accepted", func(t *testing.T) {
		content := []byte("Sample valid document content")
		req, err := createMultipartRequest("/api/v1/documents", "file", "guide.txt", content)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusAccepted, rec.Code)

		var resp DocumentResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "guide.txt", resp.Filename)
		assert.Equal(t, int64(len(content)), resp.SizeBytes)
		assert.Equal(t, "queued", resp.Status)
		assert.NotEqual(t, uuid.Nil, resp.ID)
	})

	t.Run("successful .md upload returns 202 Accepted", func(t *testing.T) {
		content := []byte("# Markdown Title\nParagraph content.")
		req, err := createMultipartRequest("/api/v1/documents", "file", "spec.md", content)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusAccepted, rec.Code)
		var resp DocumentResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "spec.md", resp.Filename)
	})

	t.Run("missing file form field returns 400", func(t *testing.T) {
		req, err := createMultipartRequest("/api/v1/documents", "wrong_field", "spec.md", []byte("content"))
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var errResp ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, string(domain.CodeValidationFailed), errResp.Error.Code)
	})

	t.Run("unsupported media type returns 415", func(t *testing.T) {
		mockSvc.uploadFn = func(ctx context.Context, ownerID string, filename string, content []byte) (*domain.Document, error) {
			return nil, domain.NewUnsupportedMediaTypeError("Unsupported file type. Only .txt and .md files are supported.")
		}

		req, err := createMultipartRequest("/api/v1/documents", "file", "archive.zip", []byte("zipdata"))
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
		var errResp ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, string(domain.CodeUnsupportedMediaType), errResp.Error.Code)
	})

	t.Run("payload exceeding limit returns 413", func(t *testing.T) {
		oversized := make([]byte, 2048)
		req, err := createMultipartRequest("/api/v1/documents", "file", "large.txt", oversized)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
		var errResp ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, string(domain.CodePayloadTooLarge), errResp.Error.Code)
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		unauthRouter := setupTestDocumentRouter(mockSvc, "", 1024)
		req, err := createMultipartRequest("/api/v1/documents", "file", "guide.txt", []byte("valid text"))
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		unauthRouter.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestDocumentHandler_List(t *testing.T) {
	docID := uuid.New()
	mockSvc := &mockDocService{
		listFn: func(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Document, *string, error) {
			next := "sample-next-cursor"
			return []*domain.Document{
				{
					ID:        docID,
					OwnerID:   ownerID,
					Filename:  "listed.txt",
					SizeBytes: 100,
					Status:    "ready",
					CreatedAt: time.Now().UTC(),
					UpdatedAt: time.Now().UTC(),
				},
			}, &next, nil
		},
	}

	router := setupTestDocumentRouter(mockSvc, "user_clerk_test", 1024)

	t.Run("list with default pagination", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/documents?limit=20", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp DocumentListResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Items, 1)
		assert.Equal(t, docID, resp.Items[0].ID)
		assert.Equal(t, "sample-next-cursor", *resp.NextCursor)
	})

	t.Run("invalid limit parameter returns 400", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/documents?limit=999", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var errResp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, string(domain.CodeValidationFailed), errResp.Error.Code)
	})
}

func TestDocumentHandler_GetByID(t *testing.T) {
	docID := uuid.New()
	mockSvc := &mockDocService{
		getByIDFn: func(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Document, error) {
			if id == docID && ownerID == "user_clerk_test" {
				return &domain.Document{
					ID:        docID,
					OwnerID:   ownerID,
					Filename:  "found.md",
					SizeBytes: 200,
					Status:    "ready",
					CreatedAt: time.Now().UTC(),
					UpdatedAt: time.Now().UTC(),
				}, nil
			}
			return nil, domain.NewNotFoundError("Document not found.")
		},
	}

	router := setupTestDocumentRouter(mockSvc, "user_clerk_test", 1024)

	t.Run("existing document returns 200", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/documents/%s", docID.String()), nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp DocumentResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, docID, resp.ID)
		assert.Equal(t, "found.md", resp.Filename)
	})

	t.Run("nonexistent document returns 404", func(t *testing.T) {
		randomID := uuid.New()
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/documents/%s", randomID.String()), nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		var errResp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, string(domain.CodeNotFound), errResp.Error.Code)
	})

	t.Run("invalid UUID param returns 400", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/documents/not-a-uuid", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var errResp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, string(domain.CodeValidationFailed), errResp.Error.Code)
	})
}

func TestDocumentHandler_Delete(t *testing.T) {
	docID := uuid.New()
	mockSvc := &mockDocService{
		deleteFn: func(ctx context.Context, ownerID string, id uuid.UUID) error {
			if id == docID && ownerID == "user_clerk_test" {
				return nil
			}
			return domain.NewNotFoundError("Document not found.")
		},
	}

	router := setupTestDocumentRouter(mockSvc, "user_clerk_test", 1024)

	t.Run("successful delete returns 204 No Content", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/documents/%s", docID.String()), nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("cross-tenant delete returns 404", func(t *testing.T) {
		otherID := uuid.New()
		req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/documents/%s", otherID.String()), nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		var errResp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, string(domain.CodeNotFound), errResp.Error.Code)
	})
}
