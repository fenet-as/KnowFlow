-- +goose Up
CREATE TABLE documents (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id      text        NOT NULL,
  filename      text        NOT NULL CHECK (char_length(filename) BETWEEN 1 AND 255),
  size_bytes    bigint      NOT NULL CHECK (size_bytes > 0),
  content_hash  text        NOT NULL,
  raw_content   text        NOT NULL,
  status        text        NOT NULL DEFAULT 'queued'
                CHECK (status IN ('queued','processing','ready','failed')),
  error_message text,
  chunk_count   integer     NOT NULL DEFAULT 0 CHECK (chunk_count >= 0),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX documents_owner_created_idx
  ON documents (owner_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE IF EXISTS documents;

