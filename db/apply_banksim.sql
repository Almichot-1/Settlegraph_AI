\set ON_ERROR_STOP on
BEGIN;
CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());
LOCK TABLE schema_migrations IN EXCLUSIVE MODE;
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='001')
    AND to_regclass('public.sim_institutions') IS NOT NULL
    AND (SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN
      ('sim_institutions','sim_accounts','sim_scenarios','sim_transfers','sim_journals','sim_entries','sim_events','sim_idempotency'))<>8
    THEN RAISE EXCEPTION 'partial BankSim baseline schema';
 END IF;
END $$;
SELECT NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='001')
   AND to_regclass('public.sim_institutions') IS NULL AS apply_migration \gset
\if :apply_migration
\i /migrations/001_banksim.sql
\endif
INSERT INTO schema_migrations(version) VALUES ('001') ON CONFLICT DO NOTHING;
SELECT NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='002') AS apply_migration \gset
\if :apply_migration
\i /migrations/002_banksim.sql
\endif
INSERT INTO schema_migrations(version) VALUES ('002') ON CONFLICT DO NOTHING;
COMMIT;
