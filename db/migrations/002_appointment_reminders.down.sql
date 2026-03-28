ALTER TABLE appointments DROP COLUMN IF EXISTS reminded_1h;
ALTER TABLE appointments DROP COLUMN IF EXISTS reminded_15m;
ALTER TABLE appointments ADD COLUMN reminded BOOLEAN DEFAULT false;
