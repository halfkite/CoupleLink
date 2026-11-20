ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_body_check;
ALTER TABLE messages ADD CONSTRAINT messages_body_check
  CHECK ((char_length(body) BETWEEN 1 AND 4000) OR (image_id IS NOT NULL AND char_length(body) = 0));