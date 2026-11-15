CREATE TABLE IF NOT EXISTS access_keys (
  id uuid PRIMARY KEY,
  key_hash text NOT NULL UNIQUE,
  key_hint text NOT NULL,
  couple_id uuid NOT NULL REFERENCES couples(id) ON DELETE CASCADE,
  label text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  last_used_at timestamptz
);
CREATE INDEX IF NOT EXISTS access_keys_group ON access_keys(couple_id);