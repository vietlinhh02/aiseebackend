-- db/migrations/003_phase3.up.sql

-- Alter existing volunteers table (created in 001_init with different defaults)
ALTER TABLE volunteers
    ALTER COLUMN rating SET NOT NULL,
    ALTER COLUMN rating SET DEFAULT 0,
    ALTER COLUMN available SET NOT NULL,
    ALTER COLUMN total_calls SET NOT NULL,
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Reset unrated volunteers from 5.0 default to 0
UPDATE volunteers SET rating = 0 WHERE total_calls = 0;

CREATE INDEX idx_volunteers_available ON volunteers (rating DESC, total_calls ASC)
    WHERE available = true;

-- Volunteer ratings table
CREATE TABLE volunteer_ratings (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    volunteer_id UUID NOT NULL REFERENCES volunteers(user_id),
    rater_id     UUID NOT NULL REFERENCES users(id),
    match_id     TEXT NOT NULL,
    score        SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 5),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(match_id, rater_id)
);

CREATE INDEX idx_volunteer_ratings_volunteer ON volunteer_ratings (volunteer_id);

-- Location tracking on users
ALTER TABLE users
    ADD COLUMN last_lat         DOUBLE PRECISION,
    ADD COLUMN last_lng         DOUBLE PRECISION,
    ADD COLUMN last_location_at TIMESTAMPTZ;
