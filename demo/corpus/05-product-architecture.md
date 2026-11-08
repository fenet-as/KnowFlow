# KnowFlow Product Architecture & Technical Reference

## System Summary
KnowFlow is a high-performance, multi-tenant Retrieval-Augmented Generation (RAG) knowledge engine. It enables users to upload documentation, ingest content asynchronously, segment passages using markdown-aware chunking, generate vector embeddings via Google Gemini, and answer queries with verifiable citations.

## Layered Architecture
KnowFlow enforces a strict unidirectional dependency graph:
```
[ HTTP Layer (Gin Handlers & Middleware) ]
                   │
                   ▼
[ Service Layer (Orchestration & Business Logic) ]
         │                   │
         ▼                   ▼
[ Repositories (PostgreSQL) ]   [ External Clients (Gemini, Clerk) ]
```

### Key Architectural Tenets
1. **Transport Isolation:** Service layer components never import Gin or HTTP packages, ensuring complete transport agnosticism.
2. **Explicit Owner Scoping:** Every database query explicitly validates the authenticated caller's `owner_id`. Unauthorized cross-tenant access returns 404 (Not Found) rather than 403 (Forbidden), eliminating resource existence reconnaissance.
3. **Deterministic Fakes:** Domain interfaces (`Embedder`, `LLM`, `Chunker`) allow 100% offline, deterministic unit testing without live network or external API credentials.

## Document Ingestion Pipeline
When a user uploads a `.txt` or `.md` file (up to 1 MB):
1. **Validation:** Checks UTF-8 encoding, filename length (1-255 characters), supported extension, and non-empty content.
2. **Document & Job Enqueue:** In a single database transaction, the document is saved with `status = 'queued'` and a background job is enqueued in the `jobs` table.
3. **Worker Processing:**
   - Background workers poll the `jobs` queue using `FOR UPDATE SKIP LOCKED`.
   - The document is segmented into semantic chunks (target ~650 tokens, max 800 tokens, 12% overlap) preserving markdown headings and code fences.
   - Text chunks are batched and embedded via Google Gemini `text-embedding-004` (768 dimensions) with task type `RETRIEVAL_DOCUMENT`.
   - Chunks and vectors are inserted into PostgreSQL `chunks` table using `pgvector`.
   - Document status transitions to `ready`, and the job transitions to `succeeded`.

## Retrieval & Question Answering Pipeline
When a user submits a question via `/api/v1/ask` or `/api/v1/conversations/:id/ask`:
1. **Query Embedding:** The question is embedded using Gemini with task type `RETRIEVAL_QUERY`.
2. **Vector Similarity Search:** PostgreSQL calculates cosine distance (`<=>`), returning the top-k chunks (`k = 6`) ordered by similarity (`score = 1 - cosine_distance`).
3. **Relevance Gating:** If the highest match score is below the relevance threshold ($\tau = 0.65$), retrieval short-circuits. The system immediately returns `"I could not find the answer in the provided documents."` without invoking the LLM, eliminating hallucination risk and unnecessary token costs.
4. **Prompt Construction:** If top match score $\ge 0.65$, a structured prompt is generated with numbered passages `[1], [2]`, conversation history, and anti-injection instructions treating passages strictly as data.
5. **Answer Generation & Citation Parsing:** Gemini `gemini-1.5-flash` generates the response. The citation parser extracts `[n]` markers, strips hallucinated markers, and attaches rich citation metadata (document name, heading, and text snippet) to the response.
