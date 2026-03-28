-- db/migrations/003_phase3.down.sql

ALTER TABLE users
    DROP COLUMN IF EXISTS last_lat,
    DROP COLUMN IF EXISTS last_lng,
    DROP COLUMN IF EXISTS last_location_at;

DROP TABLE IF EXISTS volunteer_ratings;
DROP INDEX IF EXISTS idx_volunteers_available;

ALTER TABLE volunteers
    DROP COLUMN IF EXISTS created_at;

ALTER TABLE volunteers
    ALTER COLUMN rating DROP NOT NULL,
    ALTER COLUMN rating SET DEFAULT 5.0,
    ALTER COLUMN available DROP NOT NULL,
    ALTER COLUMN total_calls DROP NOT NULL;
