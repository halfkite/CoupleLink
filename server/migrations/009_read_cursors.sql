CREATE TABLE IF NOT EXISTS read_cursors (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  couple_id uuid NOT NULL REFERENCES couples(id) ON DELETE CASCADE,
  through_seq bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now()
);