ALTER TABLE access_keys ADD COLUMN IF NOT EXISTS slot text;
CREATE UNIQUE INDEX IF NOT EXISTS access_keys_couple_slot_idx ON access_keys(couple_id, slot) WHERE slot IS NOT NULL;