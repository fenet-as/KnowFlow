# KnowFlow

KnowFlow is an AI Document-to-Action and Question-Answering engine powered by Go, PostgreSQL with pgvector, Google Gemini, and React.

## Layering and Architecture Rules

KnowFlow strictly enforces a unidirectional layered architecture:

```
[ HTTP Layer (Gin Handlers & Middleware) ]
                   │
                   ▼
[ Service Layer (Orchestration & Business Rules) ]
        │                   │
        ▼                   ▼
[ Repositories (PostgreSQL) ]   [ External Clients (Gemini, Clerk) ]
```

1. **Handlers:**
   - Parse and validate incoming HTTP requests.
   - Extract authentication claims (`sub` context as owner ID).
   - Call exactly one service method.
   - Map domain results or domain errors to JSON responses using standard error envelopes.
   - Handlers contain **no business logic**.

2. **Services:**
   - Enforce business logic and manage orchestration (chunking, embedding, vector retrieval, prompting).
   - Always take a standard `context.Context` as the first parameter.
   - **Crucial Rule:** Services **never import Gin** or any HTTP framework types. Services remain completely transport-agnostic.

3. **Repositories:**
   - Handle database queries and persistence using PostgreSQL and pgvector.
   - Always enforce multi-tenant isolation: every query scopes data by the authenticated `owner_id`.
   - Never expose raw database errors or SQL details to callers.

### Owner-Scoping Query Review Checklist (KF-11)
Before approving or committing any new repository method or SQL query, verify:
- [ ] **Explicit Owner Parameter:** Does the repository method take `ownerID string` as an explicit argument?
- [ ] **WHERE Clause Scoping:** Does every `SELECT`, `UPDATE`, and `DELETE` statement include `WHERE owner_id = $x`?
- [ ] **Parent Join Scoping:** For child tables without direct `owner_id` (e.g., `messages`), does the query `JOIN conversations` and filter by `conversations.owner_id = $x`?
- [ ] **404 Instead of 403:** Does an attempt to read, update, or delete a resource owned by another user return `domain.ErrNotFound` (404), never 403, preventing resource existence discovery?
- [ ] **Index Coverage:** Does the query use the tenant index (e.g. `documents_owner_created_idx`, `chunks_owner_idx`, `conversations_owner_updated_idx`)?

4. **External Interfaces:**
   - External systems (Embedder, LLM, Chunker) sit behind clean Go interfaces defined in domain packages, allowing tests to run with deterministic fakes.

5. **No Global State:**
   - Dependencies are explicitly passed via constructors and wired during startup in `cmd/api/main.go`.

---

## Directory Structure

```
KnowFlow/
├── cmd/
│   └── api/                 # Application entry point, wiring, startup & graceful shutdown
├── internal/
│   ├── config/              # Typed environment configuration and validation
│   ├── domain/              # Core domain models, interfaces, and custom errors
│   ├── http/                # Gin router, handlers, middleware, request/response DTOs
│   ├── auth/                # Clerk JWT verification middleware
│   ├── service/             # Business logic (documents, RAG, retrieval, conversations)
│   ├── repo/                # PostgreSQL repositories (pgx)
│   ├── worker/              # Background worker pool and job processing loop
│   ├── chunk/               # Markdown and plaintext chunker
│   ├── embed/               # Gemini embedder client & mock embedder
│   ├── llm/                 # Gemini generative LLM client & prompt builder
│   └── platform/            # Infrastructure primitives (DB connection pool, slog, rate limiter)
├── migrations/              # SQL schema migration files
├── web/                     # Frontend Vite + React + TypeScript single-page application
├── .env.example             # Documented template for all environment variables
└── .gitignore               # Ignored files, build targets, and secrets
```

---

## Getting Started

### Prerequisites
- Go 1.22+
- Docker and Docker Compose
- Node.js 20+ (for `web/`)
- Clerk account and Google Gemini API key

### Running the API locally
```bash
# Copy environment file
cp .env.example .env

# Build backend
go build ./...

# Run backend API
go run ./cmd/api
```

