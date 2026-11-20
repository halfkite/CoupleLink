ALTER TABLE access_keys ADD COLUMN IF NOT EXISTS key_plain text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS gender text NOT NULL DEFAULT '';
ALTER TABLE couples ADD COLUMN IF NOT EXISTS remark_a text NOT NULL DEFAULT '';
ALTER TABLE couples ADD COLUMN IF NOT EXISTS remark_b text NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS admin_users (
  id uuid PRIMARY KEY,
  username text NOT NULL UNIQUE,
  password_hash text NOT NULL,
  must_change boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS admin_sessions (
  token_hash text PRIMARY KEY,
  admin_id uuid NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS admin_sessions_owner ON admin_sessions(admin_id);