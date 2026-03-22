ALTER TABLE deployment_jobs ADD COLUMN IF NOT EXISTS worker_name TEXT;
ALTER TABLE deployment_jobs ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ;
