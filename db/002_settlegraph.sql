ALTER TABLE reconciliation_runs ADD COLUMN job_key text;
CREATE UNIQUE INDEX reconciliation_job_dedup ON reconciliation_runs(organization_id,job_key) WHERE job_key IS NOT NULL;
