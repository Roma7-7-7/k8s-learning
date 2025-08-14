-- Remove delay_ms column and its index
DROP INDEX IF EXISTS idx_jobs_delay_ms;
ALTER TABLE jobs DROP COLUMN IF EXISTS delay_ms;