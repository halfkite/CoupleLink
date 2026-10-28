CREATE TABLE interactions (
 id uuid PRIMARY KEY,
 couple_id uuid NOT NULL REFERENCES couples(id),
 sender_id uuid NOT NULL REFERENCES users(id),
 recipient_id uuid NOT NULL REFERENCES users(id),
 action_id text NOT NULL CHECK(action_id IN ('poke','miss_you','hug','ring','vibrate')),
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 claimed_device uuid REFERENCES devices(id),
 claimed_at timestamptz,
 result text CHECK(result IN ('requested','suppressed','unsupported','failed')),
 result_at timestamptz
);
CREATE INDEX interaction_recipient_pending ON interactions(recipient_id,couple_id,created_at) WHERE claimed_at IS NULL;
CREATE INDEX interaction_sender_recent ON interactions(sender_id,created_at DESC);
