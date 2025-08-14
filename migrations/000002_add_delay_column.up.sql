-- Add delay_ms column for stress testing
ALTER TABLE jobs ADD COLUMN delay_ms INTEGER DEFAULT 0;

-- Add index for delay_ms for potential sorting/filtering
CREATE INDEX IF NOT EXISTS idx_jobs_delay_ms ON jobs(delay_ms);