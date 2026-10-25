-- Retain the evidence that was actually supplied to each asynchronous analysis.
-- Historical offline rows have no reconstructed snapshot and remain readable.
ALTER TABLE ai_investigations ADD COLUMN evidence_snapshot jsonb;
ALTER TABLE ai_investigations ADD COLUMN actor_sub text;
ALTER TABLE ai_investigations ADD COLUMN model_id text;
ALTER TABLE ai_investigations ADD COLUMN prompt_version text;
ALTER TABLE ai_investigations ADD COLUMN invocation_id text;
ALTER TABLE ai_investigations ADD COLUMN latency_ms bigint;
ALTER TABLE ai_investigations ADD COLUMN input_tokens integer;
ALTER TABLE ai_investigations ADD COLUMN output_tokens integer;
ALTER TABLE ai_investigations ADD COLUMN validation_status text;
ALTER TABLE ai_investigations ADD COLUMN failure_code text;
CREATE FUNCTION protect_investigation_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.organization_id IS DISTINCT FROM OLD.organization_id
 OR NEW.case_id IS DISTINCT FROM OLD.case_id
 OR NEW.evidence_snapshot IS DISTINCT FROM OLD.evidence_snapshot
 OR NEW.evidence_snapshot_hash IS DISTINCT FROM OLD.evidence_snapshot_hash
 OR NEW.provider IS DISTINCT FROM OLD.provider
 THEN RAISE EXCEPTION 'investigation provenance is immutable'; END IF;
 IF OLD.status IN ('COMPLETED','REJECTED','FAILED') AND NEW IS DISTINCT FROM OLD
 THEN RAISE EXCEPTION 'terminal investigations are immutable'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER investigation_snapshot_immutable BEFORE UPDATE ON ai_investigations
 FOR EACH ROW EXECUTE FUNCTION protect_investigation_snapshot();
CREATE INDEX case_events_paging ON case_events(organization_id,case_id,version,id);
