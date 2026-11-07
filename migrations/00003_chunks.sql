-- +goose Up
CREATE TABLE chunks (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id  uuid        NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  owner_id     text        NOT NULL,
  chunk_index  integer     NOT NULL CHECK (chunk_index >= 0),
  content      text        NOT NULL,
  heading      text,
  token_count  integer     NOT NULL CHECK (token_count > 0),
  embedding    vector(768) NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (document_id, chunk_index)
);

CREATE INDEX chunks_owner_idx ON chunks (owner_id);

-- +goose Down
DROP TABLE IF EXISTS chunks;

