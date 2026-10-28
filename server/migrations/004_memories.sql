CREATE TABLE anniversaries (
  id uuid PRIMARY KEY, couple_id uuid NOT NULL REFERENCES couples(id),
  title text NOT NULL CHECK (length(title) BETWEEN 1 AND 80),
  date date NOT NULL, recurrence text NOT NULL CHECK (recurrence IN ('none','yearly')),
  reminder_time text NOT NULL DEFAULT '', timezone text NOT NULL,
  next_reminder timestamptz, version bigint NOT NULL DEFAULT 1,
  deleted_at timestamptz
);
CREATE INDEX anniversaries_couple ON anniversaries(couple_id) WHERE deleted_at IS NULL;
CREATE INDEX anniversaries_due ON anniversaries(next_reminder) WHERE deleted_at IS NULL AND next_reminder IS NOT NULL;
CREATE TABLE anniversary_reminders (
  id uuid PRIMARY KEY, anniversary_id uuid NOT NULL REFERENCES anniversaries(id),
  version bigint NOT NULL, scheduled_at timestamptz NOT NULL,
  UNIQUE(anniversary_id,version,scheduled_at)
);
CREATE TABLE anniversary_receipts (
  reminder_id uuid NOT NULL REFERENCES anniversary_reminders(id),
  device_id uuid NOT NULL REFERENCES devices(id),
  PRIMARY KEY(reminder_id,device_id)
);
