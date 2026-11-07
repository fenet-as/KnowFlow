-- +goose Up
CREATE TABLE conversations (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id   text        NOT NULL,
  title      text        NOT NULL DEFAULT 'New conversation',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX conversations_owner_updated_idx
  ON conversations (owner_id, updated_at DESC, id DESC);

CREATE TABLE messages (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  conversation_id uuid        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  role            text        NOT NULL CHECK (role IN ('user','assistant')),
  content         text        NOT NULL,
  citations       jsonb       NOT NULL DEFAULT '[]'::jsonb,
  created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX messages_conversation_created_idx
  ON messages (conversation_id, created_at, id);

-- +goose Down
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;

