ALTER TABLE source_status_snapshots ADD COLUMN scenario_run_id uuid;
ALTER TABLE source_status_snapshots ADD CONSTRAINT source_status_scenario_fk FOREIGN KEY(organization_id,scenario_run_id) REFERENCES scenario_runs(organization_id,id);
CREATE INDEX source_coverage_by_scenario ON source_status_snapshots(organization_id,scenario_run_id,provider_id,observed_at DESC);
