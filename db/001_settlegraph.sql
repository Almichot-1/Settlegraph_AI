-- SettleGraph product DB. PostgreSQL 15+ recommended. Synthetic financial records only.
-- Apply to *product* DB only, never BankSim. Every tenant scoped primary resource carries organization_id.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TABLE organizations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL,
 mode text NOT NULL CHECK (mode IN ('DEMO','PRODUCTION')), created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE app_users (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), auth_subject text NOT NULL UNIQUE,
 display_name text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE memberships (
 organization_id uuid NOT NULL REFERENCES organizations(id), user_id uuid NOT NULL REFERENCES app_users(id),
 role text NOT NULL CHECK (role IN ('OWNER','ANALYST','REVIEWER','VIEWER')),
 active boolean NOT NULL DEFAULT true, PRIMARY KEY(organization_id,user_id));
CREATE TABLE idempotency_requests (
 organization_id uuid NOT NULL REFERENCES organizations(id), actor_sub text NOT NULL,
 method text NOT NULL, route_template text NOT NULL, idempotency_key text NOT NULL,
 request_sha256 text NOT NULL, response_status integer, response_body jsonb,
 processing_state text NOT NULL DEFAULT 'STARTED' CHECK(processing_state IN ('STARTED','COMPLETED')),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id, actor_sub, method, route_template, idempotency_key));
CREATE TABLE provider_sources (
 organization_id uuid NOT NULL REFERENCES organizations(id), id text NOT NULL,
 label text NOT NULL, source_type text NOT NULL CHECK(source_type IN ('BANK','WALLET','GATEWAY')),
 sync_status text NOT NULL DEFAULT 'NEVER_SYNCED' CHECK(sync_status IN ('HEALTHY','STALE','OUTAGE','NEVER_SYNCED')),
 last_observed_at timestamptz, cursor_value text, PRIMARY KEY(organization_id,id));
CREATE TABLE source_status_snapshots (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 provider_id text NOT NULL, source_status text NOT NULL CHECK(source_status IN ('HEALTHY','OUTAGE','STALE')),
 complete_through timestamptz, observed_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,id),
 FOREIGN KEY(organization_id,provider_id) REFERENCES provider_sources(organization_id,id));
CREATE INDEX source_coverage_latest ON source_status_snapshots(organization_id,provider_id,observed_at DESC);
CREATE TABLE scenario_runs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 banksim_run_id uuid UNIQUE, scenario_key text NOT NULL, seed integer NOT NULL,
 status text NOT NULL DEFAULT 'PREPARING' CHECK(status IN ('PREPARING','READY','RUNNING','COMPLETED','FAILED')),
 current_step integer NOT NULL DEFAULT 0 CHECK(current_step>=0), total_steps integer NOT NULL CHECK(total_steps>0),
 virtual_now timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,id));
CREATE TABLE merchant_orders (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 scenario_run_id uuid, external_reference text NOT NULL,
 amount_minor bigint NOT NULL CHECK(amount_minor>0), currency char(3) NOT NULL DEFAULT 'ETB' CHECK(currency='ETB'),
 created_at timestamptz NOT NULL, UNIQUE(organization_id,external_reference), UNIQUE(organization_id,id),
 FOREIGN KEY(organization_id,scenario_run_id) REFERENCES scenario_runs(organization_id,id));
CREATE TABLE inbox_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 provider_id text NOT NULL, provider_event_id uuid NOT NULL,
 source_sequence bigint NOT NULL CHECK(source_sequence>0), payload jsonb NOT NULL, payload_sha256 text NOT NULL,
 received_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,provider_id,provider_event_id), UNIQUE(organization_id,id),
 FOREIGN KEY(organization_id,provider_id) REFERENCES provider_sources(organization_id,id));
CREATE TABLE observations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 inbox_event_id uuid NOT NULL UNIQUE, provider_id text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('PAYMENT_ACCEPTED','SETTLEMENT_POSTED','REFUND_POSTED','TRANSFER_FAILED')),
 transfer_reference text NOT NULL, payment_reference text, order_reference text,
 amount_minor bigint NOT NULL CHECK(amount_minor>0), currency char(3) NOT NULL DEFAULT 'ETB' CHECK(currency='ETB'),
 occurred_at timestamptz NOT NULL, observed_at timestamptz NOT NULL DEFAULT now(),
 scenario_run_id uuid, source_sequence bigint NOT NULL CHECK(source_sequence>0),
 UNIQUE(organization_id,id),
 FOREIGN KEY(organization_id,inbox_event_id) REFERENCES inbox_events(organization_id,id),
 FOREIGN KEY(organization_id,scenario_run_id) REFERENCES scenario_runs(organization_id,id),
 FOREIGN KEY(organization_id,provider_id) REFERENCES provider_sources(organization_id,id));
CREATE INDEX observations_lookup ON observations(organization_id,kind,transfer_reference,occurred_at);
CREATE INDEX observations_case_lookup ON observations(organization_id,scenario_run_id,kind,occurred_at);
CREATE TABLE reconciliation_runs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 scenario_run_id uuid, scope text NOT NULL CHECK(scope IN ('SCENARIO_RUN','LATEST_OBSERVATIONS')),
 rules_version text NOT NULL, status text NOT NULL CHECK(status IN ('QUEUED','RUNNING','COMPLETED','FAILED')),
 snapshot_cutoff timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), completed_at timestamptz,
 matched_order_payment integer NOT NULL DEFAULT 0, matched_payment_settlement integer NOT NULL DEFAULT 0,
 unmatched_count integer NOT NULL DEFAULT 0, UNIQUE(organization_id,id),
 FOREIGN KEY(organization_id,scenario_run_id) REFERENCES scenario_runs(organization_id,id));
CREATE TABLE match_edges (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 run_id uuid NOT NULL, stage text NOT NULL CHECK(stage IN ('ORDER_PAYMENT','PAYMENT_SETTLEMENT')),
 order_id uuid, payment_observation_id uuid NOT NULL, settlement_observation_id uuid,
 allocated_minor bigint NOT NULL CHECK(allocated_minor>0), rule text NOT NULL, proof jsonb NOT NULL,
 CHECK((stage='ORDER_PAYMENT' AND order_id IS NOT NULL AND settlement_observation_id IS NULL)
    OR (stage='PAYMENT_SETTLEMENT' AND order_id IS NULL AND settlement_observation_id IS NOT NULL)),
 UNIQUE(organization_id,id),
 FOREIGN KEY(organization_id,run_id) REFERENCES reconciliation_runs(organization_id,id),
 FOREIGN KEY(organization_id,order_id) REFERENCES merchant_orders(organization_id,id),
 FOREIGN KEY(organization_id,payment_observation_id) REFERENCES observations(organization_id,id),
 FOREIGN KEY(organization_id,settlement_observation_id) REFERENCES observations(organization_id,id));
CREATE UNIQUE INDEX one_order_allocation_per_run ON match_edges(run_id,order_id) WHERE stage='ORDER_PAYMENT';
CREATE UNIQUE INDEX one_payment_allocation_per_stage_run ON match_edges(run_id,payment_observation_id,stage);
CREATE UNIQUE INDEX one_settlement_allocation_per_run ON match_edges(run_id,settlement_observation_id) WHERE stage='PAYMENT_SETTLEMENT';
CREATE TABLE investigation_cases (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 scenario_run_id uuid, kind text NOT NULL CHECK(kind IN ('SETTLEMENT_NOT_OBSERVED','AMOUNT_MISMATCH','AMBIGUOUS_MATCH','SOURCE_UNAVAILABLE')),
 fingerprint text NOT NULL, title text NOT NULL, summary text NOT NULL,
 status text NOT NULL DEFAULT 'OPEN' CHECK(status IN ('OPEN','EVIDENCE_UPDATED','RESOLVED')),
 severity text NOT NULL CHECK(severity IN ('LOW','MEDIUM','HIGH')), version integer NOT NULL DEFAULT 1 CHECK(version>0),
 assigned_to uuid, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,fingerprint), UNIQUE(organization_id,id),
 FOREIGN KEY(organization_id,scenario_run_id) REFERENCES scenario_runs(organization_id,id),
 FOREIGN KEY(organization_id,assigned_to) REFERENCES memberships(organization_id,user_id));
CREATE INDEX open_case_index ON investigation_cases(organization_id,status,updated_at DESC);
CREATE TABLE case_evidence (
 organization_id uuid NOT NULL REFERENCES organizations(id), case_id uuid NOT NULL,
 evidence_kind text NOT NULL CHECK(evidence_kind IN ('ORDER','OBSERVATION','SOURCE_STATUS')),
 evidence_id uuid NOT NULL, first_seen_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,case_id,evidence_kind,evidence_id),
 FOREIGN KEY(organization_id,case_id) REFERENCES investigation_cases(organization_id,id));
-- evidence_id is polymorphic; application service MUST verify referenced resource belongs to same tenant.
CREATE TABLE case_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 case_id uuid NOT NULL, version integer NOT NULL CHECK(version>=1), action text NOT NULL,
 actor text NOT NULL, message text NOT NULL, occurred_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,case_id,version),
 FOREIGN KEY(organization_id,case_id) REFERENCES investigation_cases(organization_id,id));
CREATE TABLE ai_investigations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 case_id uuid NOT NULL, status text NOT NULL CHECK(status IN ('QUEUED','RUNNING','COMPLETED','REJECTED','FAILED')),
 provider text NOT NULL CHECK(provider IN ('BEDROCK','OFFLINE_STUB')),
 evidence_snapshot_hash text NOT NULL, result jsonb, created_at timestamptz NOT NULL DEFAULT now(), completed_at timestamptz,
 UNIQUE(organization_id,id), FOREIGN KEY(organization_id,case_id) REFERENCES investigation_cases(organization_id,id));
CREATE TABLE jobs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 job_kind text NOT NULL CHECK(job_kind IN ('RECONCILE','INVESTIGATE','IMPORT','BACKFILL')),
 job_key text NOT NULL, payload jsonb NOT NULL, status text NOT NULL DEFAULT 'QUEUED' CHECK(status IN ('QUEUED','RUNNING','COMPLETED','FAILED')),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts>=0), next_attempt_at timestamptz NOT NULL DEFAULT now(),
 lease_owner text, lease_expires_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), completed_at timestamptz,
 UNIQUE(organization_id,job_kind,job_key), UNIQUE(organization_id,id));
CREATE INDEX jobs_claim ON jobs(status,next_attempt_at) WHERE status IN ('QUEUED','RUNNING');
CREATE TABLE outbox_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 event_type text NOT NULL, aggregate_ref text NOT NULL, payload jsonb NOT NULL,
 published_at timestamptz, attempts integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,id));
CREATE INDEX outbox_pending ON outbox_events(created_at) WHERE published_at IS NULL;
CREATE TABLE audit_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 actor text NOT NULL, action text NOT NULL, resource_type text NOT NULL, resource_id text NOT NULL,
 payload jsonb NOT NULL DEFAULT '{}', occurred_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX audit_tenant_time ON audit_events(organization_id,occurred_at DESC,id DESC);
CREATE TABLE imports (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id),
 filename text NOT NULL, source_kind text NOT NULL CHECK(source_kind IN ('ORDERS','PAYMENTS','SETTLEMENTS')),
 expected_sha256 text NOT NULL, s3_key text NOT NULL,
 status text NOT NULL CHECK(status IN ('AWAITING_UPLOAD','QUEUED','PROCESSING','COMPLETED','FAILED')),
 accepted_records integer NOT NULL DEFAULT 0, rejected_records integer NOT NULL DEFAULT 0,
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(organization_id,id));
-- IMPORTANT: DB user permissions should be least-privileged; product credentials have no BankSim rights.
-- Invariants spanning rows, case version writes and event immutability remain explicit application transactions/tests.
