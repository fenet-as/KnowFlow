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

