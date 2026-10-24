\set ON_ERROR_STOP on
BEGIN;
CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());
LOCK TABLE schema_migrations IN EXCLUSIVE MODE;
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='001')
    AND to_regclass('public.organizations') IS NOT NULL
    AND (SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN
              ('organizations','app_users','memberships','idempotency_requests','provider_sources','source_status_snapshots','scenario_runs','merchant_orders','inbox_events','observations','reconciliation_runs','match_edges','investigation_cases','case_evidence','case_events','ai_investigations','jobs','outbox_events','audit_events','imports'))<>20
    THEN RAISE EXCEPTION 'partial product baseline schema';
 END IF;
END $$;
SELECT NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='001')
   AND to_regclass('public.organizations') IS NULL AS apply_migration \gset
\if :apply_migration
\i /migrations/001_settlegraph.sql
\endif
INSERT INTO schema_migrations(version) VALUES ('001') ON CONFLICT DO NOTHING;
SELECT NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='002') AS apply_second \gset
\if :apply_second
\i /migrations/002_settlegraph.sql
\endif
INSERT INTO schema_migrations(version) VALUES ('002') ON CONFLICT DO NOTHING;
SELECT NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='003') AS apply_third \gset
\if :apply_third
\i /migrations/003_settlegraph.sql
\endif
INSERT INTO schema_migrations(version) VALUES ('003') ON CONFLICT DO NOTHING;
SELECT NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='004') AS apply_fourth \gset
\if :apply_fourth
\i /migrations/004_settlegraph.sql
\endif
INSERT INTO schema_migrations(version) VALUES ('004') ON CONFLICT DO NOTHING;
SELECT NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='005') AS apply_fifth \gset
\if :apply_fifth
\i /migrations/005_settlegraph.sql
\endif
INSERT INTO schema_migrations(version) VALUES ('005') ON CONFLICT DO NOTHING;
COMMIT;
