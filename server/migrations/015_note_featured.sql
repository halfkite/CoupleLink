ALTER TABLE notes ADD COLUMN IF NOT EXISTS featured boolean NOT NULL DEFAULT false;
CREATE UNIQUE INDEX IF NOT EXISTS notes_featured_per_couple ON notes(couple_id) WHERE featured;
