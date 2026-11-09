# KnowFlow

[![CI](https://github.com/fenet-as/KnowFlow/actions/workflows/ci.yml/badge.svg)](https://github.com/fenet-as/KnowFlow/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/postgres-16%20%2B%20pgvector-336791?logo=postgresql)](https://github.com/pgvector/pgvector)
[![React](https://img.shields.io/badge/react-18-61DAFB?logo=react)](https://react.dev/)
[![Tailwind CSS](https://img.shields.io/badge/tailwind-3.4-38B2AC?logo=tailwindcss)](https://tailwindcss.com/)

**KnowFlow** is an enterprise-grade, multi-tenant Retrieval-Augmented Generation (RAG) knowledge engine. It enables users to upload documentation (`.md` and `.txt`), process text into semantic chunks asynchronously via background worker pools, generate dense vector embeddings via Google Gemini, and answer questions grounded strictly in the user's private documents with verifiable citations.

---

## Table of Contents
- [Architecture & Layering Rules](#architecture--layering-rules)
- [Key Architectural Decisions](#key-architectural-decisions)
- [Specification Documents](#specification-documents)
- [Prerequisites](#prerequisites)
- [Quick Start with Docker Compose](#quick-start-with-docker-compose)
- [Local Development](#local-development)
- [Environment Variables](#environment-variables)
- [Testing](#testing)
- [RAG Evaluation & Threshold Tuning](#rag-evaluation--threshold-tuning)
- [API Reference](#api-reference)
- [Known Limitations & Roadmap](#known-limitations--roadmap)

---

## Architecture & Layering Rules

KnowFlow strictly enforces a unidirectional layered architecture:

```
[ HTTP Layer (Gin Handlers & Middleware) ]
                   │
                   ▼
[ Service Layer (Orchestration & Business Logic) ]
         │                   │
         ▼                   ▼
[ Repositories (PostgreSQL) ]   [ External Clients (Gemini) ]
```

1. **HTTP Handlers (`internal/http/`):**
   - Parse and validate incoming requests, extract verified auth claims (`owner_id` from JWT access token).
   - Delegate directly to a single service method.
   - Return standard success envelopes (`{"data": ...}`) or error envelopes (`{"error": {"code": "...", "message": "..."}}`).
   - Handlers contain **zero business logic**.

2. **Service Layer (`internal/service/`):**
   - Enforce domain rules, transactional integrity, and orchestration (chunking, embedding, vector search, prompt assembly, and citation parsing).
   - All methods accept `context.Context` as the first argument.
   - **Crucial Rule:** Services **never import Gin** or HTTP framework packages, keeping business logic transport-agnostic and 100% unit-testable.

3. **Repositories (`internal/repo/`):**
   - Manage persistence against PostgreSQL 16 using `pgxpool`.
   - **Multi-Tenant Isolation:** Every single query enforces `WHERE owner_id = $x`. Accessing another tenant's resource returns `domain.ErrNotFound` (`404`), never `403`, eliminating resource existence enumeration.

4. **Background Worker (`internal/worker/`):**
   - Autonomous worker pool polling PostgreSQL `jobs` table using `FOR UPDATE SKIP LOCKED`.
   - Decouples synchronous HTTP file upload from compute-heavy chunking and vector embedding API calls.
   - Backoff with jitter on transient errors (rate limits, upstream timeouts) and automatic recovery of crashed/stale jobs.

---

## Key Architectural Decisions

| Decision | Selection | Rationale |
| --- | --- | --- |
| **Language & Web Framework** | Go 1.22 + Gin | Compile-time safety, high concurrency, minimal memory footprint, and standard HTTP routing. |
| **Vector Storage** | PostgreSQL 16 + `pgvector` | Eliminates the operational complexity and eventual consistency hazards of managing a standalone vector database. Metadata, documents, jobs, and vector embeddings reside in a single ACID-compliant database. |
| **Background Processing** | Row-locked Postgres Queue (`FOR UPDATE SKIP LOCKED`) | Eliminates Redis/RabbitMQ operational overhead. Workers safely dequeue jobs concurrently with zero duplicate claims and zero lock contention. |
| **Text Segmentation** | Markdown-Aware Chunker | Preserves headings, sections, and code fences. Avoids mid-sentence or mid-code splitting while maintaining an overlap window (~12%) for semantic continuity. |
| **Relevance Gating** | Cosine Similarity Gate ($\tau = 0.65$) | Prevents hallucination by rejecting out-of-domain questions *before* LLM generation. Reduces API token spend to zero when relevant documentation is missing. |
| **Citation Extraction** | Marker Parser (`[n]`) + Bound Checking | Strips hallucinated markers. If the LLM generates an answer without valid grounded markers, the system falls back to the fixed refusal message. |
| **Multi-Tenant Security** | Strict 404 on Cross-Tenant Access | Prevents attackers from probing whether document or conversation IDs exist for other users. |

---

## Specification Documents

For complete deep-dive documentation on requirements, schemas, and subsystem design:

- 📋 [**Requirements & Use Cases**](docs/KnowFlow%20Requirements%20&%20Use%20Cases.md): Business goals, user stories, personas, and quality attributes.
- ⚙️ [**Backend Guide**](docs/KnowFlow%20Backend%20Guide.md): Detailed backend service contracts, error handling standards, and worker specifications.
- 🗄️ [**Database Guide**](docs/KnowFlow%20Database%20Guide.md): Relational schema, indices, migration workflows, and pgvector tuning.
- 🎨 [**Frontend Guide**](docs/KnowFlow%20Frontend%20Guide.md): React component tree, native auth integration, state machines, and UX guidelines.
- 🎯 [**Execution Backlog**](docs/KnowFlow%20Execution%20Backlog%20(v1).md): Complete phase-by-phase implementation issues and acceptance criteria.
- 🔬 [**RAG Evaluation & Threshold Tuning**](docs/KnowFlow%20RAG%20Evaluation%20&%20Threshold%20Tuning.md): 20-question evaluation benchmark, $k=6$ selection, and $\tau=0.65$ threshold analysis.

---

## Prerequisites

- **Go:** 1.22 or higher
- **Node.js:** 20+ and npm (for frontend single-page application)
- **Docker & Docker Compose:** For running PostgreSQL with pgvector and containerized deployments
- **Google Gemini API Key:** For embeddings (`gemini-embedding-001`) and generative answers (`gemini-3.5-flash`)
- **JWT Secret:** For self-hosted native authentication (`JWT_SECRET`, min 32 characters)

---

## Quick Start with Docker Compose

The fastest way to spin up the entire KnowFlow stack (PostgreSQL + pgvector, Go API, and React Frontend):

1. **Clone the repository:**
   ```bash
   git clone https://github.com/fenet-as/KnowFlow.git
   cd KnowFlow
   ```

2. **Configure environment:**
   ```bash
   cp .env.example .env
   # Edit .env and supply your GEMINI_API_KEY and JWT_SECRET
   ```

3. **Start all services:**
   ```bash
   docker compose up -d --build
   ```

4. **Verify running containers:**
   - **Frontend UI:** [http://localhost:3000](http://localhost:3000)
   - **Backend API:** [http://localhost:8080/healthz](http://localhost:8080/healthz)
   - **PostgreSQL:** `localhost:5433` (mapped from container 5432)

5. **Stop services:**
   ```bash
   docker compose down
   ```

---

## Local Development

### 1. Database Setup
Start only the PostgreSQL container:
```bash
docker compose up -d postgres
```

Run database migrations:
```bash
make migrate-up
# Or: go run ./cmd/migrate -db "postgres://knowflow:knowflow@localhost:5433/knowflow?sslmode=disable" -dir "migrations" up
```

### 2. Run Backend API
```bash
make run
# Or: go run ./cmd/api
```
The server will start on `http://localhost:8080`.

### 3. Run Frontend Single-Page Application
In a separate terminal:
```bash
cd web
npm install
npm run dev
```
The Vite development server will start at `http://localhost:5173`.

---

## Environment Variables

| Variable | Description | Default | Required |
| --- | --- | --- | --- |
| `HTTP_ADDR` | Listening host and port for HTTP server | `:8080` | No |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://knowflow:knowflow@localhost:5433/knowflow?sslmode=disable` | **Yes** |
| `JWT_SECRET` | Signing secret for native JWT access tokens (min 32 chars) | - | **Yes** |
| `ACCESS_TOKEN_DURATION` | Lifespan of JWT access tokens | `15m` | No |
| `REFRESH_TOKEN_DURATION` | Lifespan of rotatable refresh tokens | `168h` | No |
| `CLERK_ISSUER` | Optional legacy Clerk JWT issuer URL fallback | - | No |
| `CLERK_AUTHORIZED_PARTIES` | Comma-separated list of allowed `azp` origins | `http://localhost:5173,http://localhost:3000` | No |
| `GEMINI_API_KEY` | Google Gemini API key | - | **Yes** |
| `EMBEDDING_MODEL` | Gemini model for dense vector embeddings | `gemini-embedding-001` | No |
| `EMBEDDING_DIM` | Dense vector embedding dimensionality | `768` | No |
| `LLM_MODEL` | Gemini generative model for answers | `gemini-3.5-flash` | No |
| `RETRIEVAL_TOP_K` | Number of chunks retrieved per query ($k$) | `6` | No |
| `RETRIEVAL_MIN_SCORE` | Relevance similarity score gate threshold ($\tau$) | `0.65` | No |
| `WORKER_COUNT` | Number of background worker routines | `4` | No |
| `WORKER_POLL_INTERVAL` | Queue polling interval | `1s` | No |
| `CORS_ALLOWED_ORIGINS` | Comma-separated list of allowed web origins | `http://localhost:5173,http://localhost:3000` | No |
| `MAX_UPLOAD_BYTES` | Maximum file upload size in bytes | `1048576` (1 MB) | No |
| `LOG_LEVEL` | Logging verbosity (`debug`, `info`, `warn`, `error`) | `info` | No |

---

## Testing

KnowFlow includes comprehensive unit and integration test suites:

```bash
# Run all unit and integration tests with coverage reporting
make test
# Or: go test -v -p 1 -cover ./...

# Run the integration test suite against PostgreSQL
make test-integration
# Or: go test -v -p 1 ./test/integration/...

# Generate statement coverage report
make test-coverage
# Or: go test -v -p 1 -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```

### Test Suite Highlights
- **Unit Tests (`internal/`):** Chunker boundary edge cases, prompt injection sanitization, citation parsing, error classification table, and auth verification.
- **Integration Tests (`test/integration/`):**
  - **Multi-Tenant Isolation:** Proves User A cannot read, list, search, or delete User B's resources.
  - **Concurrent Claiming:** 20 jobs claimed by 5 concurrent worker routines with zero duplicates.
  - **Idempotent Ingestion:** Re-ingesting a document yields identical chunks and state.
  - **Delete Cascade:** Deleting a document removes its chunks and jobs; deleting a conversation removes its messages.
  - **E2E Pipeline:** Upload $\to$ Claim $\to$ Ingest $\to$ Vector search $\to$ Grounded answer with citations.

---

## RAG Evaluation & Threshold Tuning

KnowFlow provides a curated 5-document enterprise corpus in `demo/corpus/` and a 20-question evaluation benchmark spanning direct lookups, multi-document synthesis, semantic paraphrasing, and unanswerable questions.

Run the evaluation benchmark:
```bash
go run ./cmd/eval -corpus "demo/corpus" -k 6 -threshold 0.65
```

Detailed tuning findings are documented in [**KnowFlow RAG Evaluation & Threshold Tuning**](docs/KnowFlow%20RAG%20Evaluation%20&%20Threshold%20Tuning.md).

---

## Demo Seed Data & Presentation Script

To quickly prepare a demonstration environment with pre-ingested enterprise documents:
```bash
# Seed the demo corpus into PostgreSQL
make seed
# Or: go run ./cmd/seed -dir "demo/corpus"
```

A comprehensive live demonstration script featuring a pre-demo checklist, five curated demonstration questions (highlighting single-document lookup, multi-hop cross-document reasoning, conversational context, out-of-domain refusal, and prompt injection defense), and presenter talking points is provided in [**demo/DEMO_SCRIPT.md**](demo/DEMO_SCRIPT.md).

---

## API Reference

All protected endpoints under `/api/v1` require the header:
```http
Authorization: Bearer <jwt_access_token>
```

### 1. Authentication (`/api/v1/auth`)

#### `POST /api/v1/auth/register`
Creates a new user account, mints a JWT access token, and sets an `HttpOnly` refresh token cookie.
- **Request:**
  ```json
  {
    "email": "user@example.com",
    "password": "Password123!",
    "full_name": "Jane Doe"
  }
  ```
- **Response (201 Created):**
  ```json
  {
    "user": {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "email": "user@example.com",
      "full_name": "Jane Doe",
      "role": "member",
      "created_at": "2026-11-10T00:00:00Z",
      "updated_at": "2026-11-10T00:00:00Z"
    },
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 900
  }
  ```

#### `POST /api/v1/auth/login`
Authenticates credentials, returns an access token, and sets an `HttpOnly` refresh token cookie.

#### `POST /api/v1/auth/refresh`
Rotates the session using the `knowflow_refresh_token` cookie and issues a new access token.

#### `POST /api/v1/auth/logout`
Revokes the active refresh token and clears the cookie.

#### `GET /api/v1/auth/me` *(Protected)*
Returns the authenticated caller's profile.

### 2. Health & System

#### `GET /healthz`
Liveness probe.
- **Response (200 OK):**
  ```json
  { "status": "ok" }
  ```

#### `GET /readyz`
Readiness probe verifying PostgreSQL connection.
- **Response (200 OK):**
  ```json
  { "status": "ok", "db": "healthy" }
  ```

#### `GET /version`
Build and commit metadata.
- **Response (200 OK):**
  ```json
  {
    "version": "v1.0.0",
    "commit": "a0501b1",
    "build_time": "2026-11-08T17:00:00Z"
  }
  ```

---

### 3. Documents

#### `POST /api/v1/documents`
Upload a `.txt` or `.md` file for background chunking and embedding.
- **Request:** `multipart/form-data` with form field `file` (max 1 MB).
- **Response (202 Accepted):**
  ```json
  {
    "data": {
      "id": "8f8b5f90-1c42-4f36-8a71-8bc68521e102",
      "filename": "handbook.md",
      "size_bytes": 1024,
      "status": "queued",
      "chunk_count": 0,
      "error_message": null,
      "created_at": "2026-11-08T17:10:00Z",
      "updated_at": "2026-11-08T17:10:00Z"
    }
  }
  ```

#### `GET /api/v1/documents`
List documents belonging to the authenticated user.
- **Query Parameters:** `limit` (int, default 20), `cursor` (base64 string, optional).
- **Response (200 OK):**
  ```json
  {
    "data": [
      {
        "id": "8f8b5f90-1c42-4f36-8a71-8bc68521e102",
        "filename": "handbook.md",
        "size_bytes": 1024,
        "status": "ready",
        "chunk_count": 4,
        "error_message": null,
        "created_at": "2026-11-08T17:10:00Z",
        "updated_at": "2026-11-08T17:10:05Z"
      }
    ],
    "pagination": {
      "next_cursor": null,
      "has_more": false
    }
  }
  ```

#### `GET /api/v1/documents/:id`
Fetch document details and processing status (`queued`, `processing`, `ready`, `failed`).
- **Response (200 OK):**
  ```json
  {
    "data": {
      "id": "8f8b5f90-1c42-4f36-8a71-8bc68521e102",
      "filename": "handbook.md",
      "size_bytes": 1024,
      "status": "ready",
      "chunk_count": 4,
      "error_message": null,
      "created_at": "2026-11-08T17:10:00Z",
      "updated_at": "2026-11-08T17:10:05Z"
    }
  }
  ```

#### `DELETE /api/v1/documents/:id`
Delete a document and cascade deletion to its chunks and jobs.
- **Response:** `204 No Content`

---

### 4. Chunks

#### `GET /api/v1/chunks/:id`
Retrieve passage text and metadata for a citation chip.
- **Response (200 OK):**
  ```json
  {
    "data": {
      "id": "c3e981df-6b8f-4318-8f81-2c069501a357",
      "document_id": "8f8b5f90-1c42-4f36-8a71-8bc68521e102",
      "document_name": "handbook.md",
      "chunk_index": 1,
      "heading": "Paid Time Off (PTO)",
      "content": "Full-time employees accrue 20 days of paid vacation per calendar year..."
    }
  }
  ```

---

### 5. Direct Question Answering (Single-Turn)

#### `POST /api/v1/ask`
Submit a question across all or selected documents.
- **Request Body:**
  ```json
  {
    "question": "How many days of paid vacation do employees accrue?",
    "document_ids": ["8f8b5f90-1c42-4f36-8a71-8bc68521e102"]
  }
  ```
- **Response (200 OK - Answered):**
  ```json
  {
    "data": {
      "answer": "Full-time employees receive 20 days of paid vacation per calendar year [1]. Up to 5 unused days may roll over [1].",
      "grounded": true,
      "citations": [
        {
          "marker": 1,
          "chunk_id": "c3e981df-6b8f-4318-8f81-2c069501a357",
          "document_id": "8f8b5f90-1c42-4f36-8a71-8bc68521e102",
          "document_name": "handbook.md",
          "heading": "Paid Time Off (PTO)",
          "snippet": "Full-time employees accrue 20 days of paid vacation per calendar year..."
        }
      ]
    }
  }
  ```
- **Response (200 OK - Unanswerable / Below Relevance Gate):**
  ```json
  {
    "data": {
      "answer": "I could not find the answer in the provided documents.",
      "grounded": false,
      "citations": []
    }
  }
  ```

---

### 6. Conversations (Multi-Turn)

#### `POST /api/v1/conversations`
Create a new conversation session.
- **Request Body:**
  ```json
  {
    "title": "Benefits and Vacation Discussion"
  }
  ```
- **Response (201 Created):**
  ```json
  {
    "data": {
      "id": "1e32d189-58b2-4d7a-8fbb-7c4568912345",
      "title": "Benefits and Vacation Discussion",
      "created_at": "2026-11-08T17:20:00Z",
      "updated_at": "2026-11-08T17:20:00Z"
    }
  }
  ```

#### `GET /api/v1/conversations`
List conversations with cursor pagination.
- **Query Parameters:** `limit` (default 20), `cursor` (optional).
- **Response (200 OK):**
  ```json
  {
    "data": [
      {
        "id": "1e32d189-58b2-4d7a-8fbb-7c4568912345",
        "title": "Benefits and Vacation Discussion",
        "created_at": "2026-11-08T17:20:00Z",
        "updated_at": "2026-11-08T17:20:00Z"
      }
    ],
    "pagination": {
      "next_cursor": null,
      "has_more": false
    }
  }
  ```

#### `GET /api/v1/conversations/:id`
Fetch conversation detail and recent message history.
- **Query Parameters:** `limit` (default 50).
- **Response (200 OK):**
  ```json
  {
    "data": {
      "conversation": {
        "id": "1e32d189-58b2-4d7a-8fbb-7c4568912345",
        "title": "Benefits and Vacation Discussion",
        "created_at": "2026-11-08T17:20:00Z",
        "updated_at": "2026-11-08T17:22:00Z"
      },
      "messages": [
        {
          "id": "a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d",
          "conversation_id": "1e32d189-58b2-4d7a-8fbb-7c4568912345",
          "role": "user",
          "content": "Can I roll over unused vacation days?",
          "citations": [],
          "created_at": "2026-11-08T17:21:00Z"
        },
        {
          "id": "b2c3d4e5-f6a7-8b9c-0d1e-2f3a4b5c6d7e",
          "conversation_id": "1e32d189-58b2-4d7a-8fbb-7c4568912345",
          "role": "assistant",
          "content": "Yes, employees may carry over up to 5 unused vacation days into the following year [1].",
          "citations": [
            {
              "marker": 1,
              "chunk_id": "c3e981df-6b8f-4318-8f81-2c069501a357",
              "document_id": "8f8b5f90-1c42-4f36-8a71-8bc68521e102",
              "document_name": "handbook.md",
              "heading": "Paid Time Off (PTO)",
              "snippet": "Up to 5 unused vacation days may be rolled over into the following calendar year..."
            }
          ],
          "created_at": "2026-11-08T17:21:02Z"
        }
      ]
    }
  }
  ```

#### `POST /api/v1/conversations/:id/ask`
Submit a question in an active conversation. The server injects prior dialogue history into the context, updates the conversation title on the first question, and appends both turns.
- **Request Body:**
  ```json
  {
    "question": "What happens to days beyond those 5?",
    "document_ids": []
  }
  ```
- **Response (200 OK):**
  ```json
  {
    "data": {
      "answer": "Any unused days beyond 5 are forfeited on March 31 unless prohibited by local statutory law [1].",
      "grounded": true,
      "citations": [
        {
          "marker": 1,
          "chunk_id": "c3e981df-6b8f-4318-8f81-2c069501a357",
          "document_id": "8f8b5f90-1c42-4f36-8a71-8bc68521e102",
          "document_name": "handbook.md",
          "heading": "Paid Time Off (PTO)",
          "snippet": "Any additional unused days beyond 5 are forfeited on March 31..."
        }
      ]
    }
  }
  ```

#### `DELETE /api/v1/conversations/:id`
Delete conversation and cascade delete all its messages.
- **Response:** `204 No Content`

---

## Known Limitations & Roadmap

### Current Limitations (v1)
- **Supported Formats:** Markdown (`.md`) and plain text (`.txt`) only. Binary files (`.pdf`, `.docx`) are rejected with `415 Unsupported Media Type`.
- **Upload Size:** Capped at 1 MB per document to prevent memory spikes on small VPS nodes.
- **Search Strategy:** Exact cosine distance search. While highly accurate for thousands of chunks, HNSW index optimization will be added in v1.1 for corpora exceeding 100,000 chunks.

### Planned for v2 (Stretch Backlog)
- [ ] **Streaming Responses (SSE):** Stream tokens incrementally to the frontend.
- [ ] **Hybrid Search:** Combine pgvector dense similarity with PostgreSQL `tsvector` full-text search using Reciprocal Rank Fusion (RRF).
- [ ] **Reranking:** Cross-encoder reranker for fine-grained chunk reordering.
- [ ] **PDF & DOCX Support:** Extract text from formatted office documents.
- [ ] **Automated OpenAPI / Swagger Spec:** Auto-generated interactive API documentation.
