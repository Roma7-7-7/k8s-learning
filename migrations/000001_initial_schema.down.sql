-- Drop indexes
DROP INDEX IF EXISTS idx_jobs_worker_id;
DROP INDEX IF EXISTS idx_jobs_processing_type;
DROP INDEX IF EXISTS idx_jobs_created_at;
DROP INDEX IF EXISTS idx_jobs_status;

-- Drop jobs table
DROP TABLE IF EXISTS jobs;