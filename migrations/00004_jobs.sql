-- +goose Up
CREATE TABLE jobs (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id   uuid        NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  type          text        NOT NULL DEFAULT 'ingest_document',
  status        text        NOT NULL DEFAULT 'queued'
                CHECK (status IN ('queued','running','succeeded','failed')),
  attempts      integer     NOT NULL DEFAULT 0,
  max_attempts  integer     NOT NULL DEFAULT 5,
  run_after     timestamptz NOT NULL DEFAULT now(),
  locked_at     timestamptz,
  locked_by     text,
  last_error    text,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX jobs_claim_idx   ON jobs (run_after) WHERE status = 'queued';
CREATE INDEX jobs_running_idx ON jobs (locked_at) WHERE status = 'running';
CREATE INDEX jobs_document_idx ON jobs (document_id);

-- +goose Down
DROP TABLE IF EXISTS jobs;

