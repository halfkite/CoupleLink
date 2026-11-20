CREATE TABLE IF NOT EXISTS cycle_profiles (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  timezone text NOT NULL DEFAULT 'Asia/Shanghai',
  average_cycle_days integer NOT NULL DEFAULT 28 CHECK (average_cycle_days BETWEEN 21 AND 45),
  average_period_days integer NOT NULL DEFAULT 5 CHECK (average_period_days BETWEEN 1 AND 10),
  reminder_enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS period_records (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  start_date date NOT NULL,
  end_date date,
  flow_level text NOT NULL DEFAULT 'medium' CHECK (flow_level IN ('light','medium','heavy')),
  note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (end_date IS NULL OR end_date >= start_date)
);
CREATE UNIQUE INDEX IF NOT EXISTS period_records_start ON period_records(user_id, start_date);
CREATE INDEX IF NOT EXISTS period_records_user_start ON period_records(user_id, start_date DESC);

CREATE TABLE IF NOT EXISTS cycle_daily_logs (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  log_date date NOT NULL,
  pain_level smallint NOT NULL DEFAULT 0 CHECK (pain_level BETWEEN 0 AND 10),
  mood text NOT NULL DEFAULT '' CHECK (mood IN ('','happy','calm','tired','sad','irritable','anxious')),
  symptoms text[] NOT NULL DEFAULT '{}',
  note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS cycle_daily_logs_day ON cycle_daily_logs(user_id, log_date);

CREATE TABLE IF NOT EXISTS care_preferences (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  preferred_support text[] NOT NULL DEFAULT '{}',
  avoid_support text[] NOT NULL DEFAULT '{}',
  note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 300),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS care_share_permissions (
  id uuid PRIMARY KEY,
  owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  partner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  couple_id uuid NOT NULL REFERENCES couples(id) ON DELETE CASCADE,
  scope text NOT NULL CHECK (scope IN ('cycle_summary','period_dates','daily_logs','care_preferences')),
  granted_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  CHECK (owner_user_id <> partner_user_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS care_share_active
  ON care_share_permissions(owner_user_id, partner_user_id, scope) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS care_tasks (
  id uuid PRIMARY KEY,
  couple_id uuid NOT NULL REFERENCES couples(id) ON DELETE CASCADE,
  subject_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
  note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
  due_at timestamptz,
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','done')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);
CREATE INDEX IF NOT EXISTS care_tasks_couple ON care_tasks(couple_id, status, due_at);

CREATE TABLE IF NOT EXISTS cycle_reminders (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('period_upcoming','period_start','log_nudge','care_task')),
  scheduled_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, kind, scheduled_at)
);
CREATE INDEX IF NOT EXISTS cycle_reminders_pending ON cycle_reminders(user_id, scheduled_at);

CREATE TABLE IF NOT EXISTS cycle_reminder_receipts (
  reminder_id uuid NOT NULL REFERENCES cycle_reminders(id) ON DELETE CASCADE,
  device_id uuid NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (reminder_id, device_id)
);
