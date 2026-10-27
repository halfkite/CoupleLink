CREATE TABLE users (
  id uuid PRIMARY KEY,
  username text NOT NULL UNIQUE,
  display_name text NOT NULL,
  password_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE registration_gate (id integer PRIMARY KEY CHECK (id=1));
INSERT INTO registration_gate VALUES (1);
CREATE TABLE devices (
  id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id),
  label text NOT NULL, platform text NOT NULL,
  last_seen timestamptz NOT NULL DEFAULT now(), revoked_at timestamptz
);
CREATE TABLE sessions (
  id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id),
  device_id uuid NOT NULL REFERENCES devices(id), token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL, revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_owner_device ON sessions(user_id,device_id);
CREATE TABLE couples (
  id uuid PRIMARY KEY, user_a uuid NOT NULL REFERENCES users(id),
  user_b uuid NOT NULL REFERENCES users(id), start_at date NOT NULL,
  timezone text NOT NULL, status text NOT NULL CHECK(status IN ('active','ended')),
  version bigint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now(),
  CHECK(user_a<>user_b)
);
CREATE TABLE couple_members (
  user_id uuid PRIMARY KEY REFERENCES users(id), couple_id uuid NOT NULL REFERENCES couples(id)
);
CREATE TABLE invitations (
  id uuid PRIMARY KEY, creator_id uuid NOT NULL REFERENCES users(id),
  token_hash text NOT NULL UNIQUE, expires_at timestamptz NOT NULL,
  used_at timestamptz, revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invitations_owner ON invitations(creator_id);
CREATE TABLE sync_heads (user_id uuid PRIMARY KEY REFERENCES users(id), seq bigint NOT NULL DEFAULT 0);
CREATE TABLE sync_events (
  user_id uuid NOT NULL REFERENCES users(id), seq bigint NOT NULL,
  event_type text NOT NULL, ref_id uuid NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id,seq)
);
