ALTER TABLE couples ADD COLUMN message_seq bigint NOT NULL DEFAULT 0;
CREATE TABLE messages (
  id uuid PRIMARY KEY,
  couple_id uuid NOT NULL REFERENCES couples(id),
  sender_id uuid NOT NULL REFERENCES users(id),
  body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 4000),
  seq bigint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(couple_id, seq)
);
CREATE INDEX messages_history ON messages(couple_id, seq DESC);
CREATE TABLE message_receipts (
  message_id uuid NOT NULL REFERENCES messages(id),
  device_id uuid NOT NULL REFERENCES devices(id),
  user_id uuid NOT NULL REFERENCES users(id),
  delivered_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(message_id, device_id)
);
CREATE TABLE realtime_outbox (
  id uuid PRIMARY KEY,
  kind text NOT NULL CHECK(kind IN ('notify','disconnect')),
  target text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  attempts integer NOT NULL DEFAULT 0
);
CREATE INDEX realtime_outbox_ready ON realtime_outbox(next_attempt_at);
