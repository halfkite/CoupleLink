ALTER TABLE couples ADD COLUMN IF NOT EXISTS archived_at timestamptz;
CREATE INDEX IF NOT EXISTS couples_archived ON couples(archived_at) WHERE archived_at IS NOT NULL;