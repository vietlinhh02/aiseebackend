ALTER TABLE appointments DROP COLUMN reminded;
ALTER TABLE appointments ADD COLUMN reminded_1h BOOLEAN DEFAULT false;
ALTER TABLE appointments ADD COLUMN reminded_15m BOOLEAN DEFAULT false;
