-- BankSim only. All financial amounts are synthetic. Logical institutions share one sandbox clearing ledger.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TABLE sim_institutions (
 id text PRIMARY KEY, name text NOT NULL, kind text NOT NULL CHECK(kind IN ('BANK','WALLET')));
CREATE TABLE sim_accounts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), institution_id text REFERENCES sim_institutions(id),
 owner_label text NOT NULL, account_type text NOT NULL CHECK(account_type IN ('CUSTOMER','MERCHANT','CLEARING','EQUITY')),
 currency char(3) NOT NULL DEFAULT 'ETB' CHECK(currency='ETB'),
 balance_minor bigint NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX sim_account_institution ON sim_accounts(institution_id,account_type);
CREATE TABLE sim_scenarios (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL,
 scenario_key text NOT NULL CHECK(scenario_key IN ('normal_flow','delayed_settlement','duplicate_webhook','provider_outage')),
 seed integer NOT NULL, current_step integer NOT NULL DEFAULT 0,
 total_steps integer NOT NULL CHECK(total_steps>0), virtual_now timestamptz NOT NULL,
 status text NOT NULL CHECK(status IN ('READY','RUNNING','COMPLETED','FAILED')),
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(organization_id,id));
CREATE TABLE sim_transfers (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), scenario_id uuid REFERENCES sim_scenarios(id),
 from_account_id uuid NOT NULL REFERENCES sim_accounts(id), to_account_id uuid NOT NULL REFERENCES sim_accounts(id),
 order_reference text NOT NULL, transfer_reference text NOT NULL UNIQUE,
 amount_minor bigint NOT NULL CHECK(amount_minor>0), currency char(3) NOT NULL DEFAULT 'ETB' CHECK(currency='ETB'),
 state text NOT NULL CHECK(state IN ('CREATED','ACCEPTED','SETTLEMENT_PENDING','SETTLED','FAILED')),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK(from_account_id <> to_account_id));
CREATE TABLE sim_journals (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), transfer_id uuid REFERENCES sim_transfers(id),
 action text NOT NULL, status text NOT NULL DEFAULT 'DRAFT' CHECK(status IN ('DRAFT','POSTED')),
 currency char(3) NOT NULL DEFAULT 'ETB' CHECK(currency='ETB'),
 occurred_at timestamptz NOT NULL, UNIQUE(transfer_id,action));
CREATE TABLE sim_entries (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), journal_id uuid NOT NULL REFERENCES sim_journals(id),
 account_id uuid NOT NULL REFERENCES sim_accounts(id), signed_minor bigint NOT NULL CHECK(signed_minor<>0),
 created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX sim_entries_by_account ON sim_entries(account_id,created_at);
CREATE TABLE sim_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), scenario_id uuid NOT NULL REFERENCES sim_scenarios(id),
 provider_id text NOT NULL REFERENCES sim_institutions(id), sequence bigint NOT NULL,
 payload jsonb NOT NULL, occurred_at timestamptz NOT NULL,
 delivery_status text NOT NULL DEFAULT 'PENDING' CHECK(delivery_status IN ('PENDING','DELAYED','DELIVERED')),
 delivered_at timestamptz, attempts integer NOT NULL DEFAULT 0,
 UNIQUE(provider_id,sequence), UNIQUE(id,provider_id));
CREATE INDEX sim_events_for_provider ON sim_events(provider_id,sequence);
CREATE TABLE sim_idempotency (
 scope text NOT NULL, key text NOT NULL, request_sha256 text NOT NULL,
 response_status integer NOT NULL, response_body jsonb NOT NULL,
 PRIMARY KEY(scope,key));
-- When journal becomes POSTED, all entries must be balanced and belong to same currency.
CREATE OR REPLACE FUNCTION validate_posted_journal() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE journal_total numeric; wrong_currency integer; entry_count integer;
BEGIN
 IF OLD.status='POSTED' AND NEW.status<>'POSTED' THEN
   RAISE EXCEPTION 'posted journal cannot be reverted';
 END IF;
 IF NEW.status='POSTED' AND OLD.status <> 'POSTED' THEN
   SELECT COALESCE(SUM(e.signed_minor),0), COUNT(*),
       COUNT(*) FILTER (WHERE a.currency <> NEW.currency)
   INTO journal_total, entry_count, wrong_currency
   FROM sim_entries e JOIN sim_accounts a ON a.id=e.account_id WHERE e.journal_id=NEW.id;
   IF entry_count<2 OR journal_total<>0 OR wrong_currency<>0 THEN
     RAISE EXCEPTION 'unbalanced or invalid simulated journal %', NEW.id;
   END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER on_sim_journal_post BEFORE UPDATE OF status ON sim_journals
 FOR EACH ROW EXECUTE FUNCTION validate_posted_journal();
CREATE OR REPLACE FUNCTION prevent_posted_entry_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE st text;
BEGIN
 IF TG_OP='DELETE' THEN
   SELECT status INTO st FROM sim_journals WHERE id=OLD.journal_id;
 ELSE
   SELECT status INTO st FROM sim_journals WHERE id=NEW.journal_id;
 END IF;
 IF st='POSTED' THEN RAISE EXCEPTION 'immutable posted journal entry'; END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END $$;
CREATE TRIGGER sim_entry_no_mod BEFORE INSERT OR UPDATE OR DELETE ON sim_entries
 FOR EACH ROW EXECUTE FUNCTION prevent_posted_entry_mutation();
-- The application must lock involved accounts in stable UUID order, check payer availability,
-- insert draft journal + entries, update materialized balances, then mark POSTED within one TX.
-- Cross-institution transfer is a sandbox clearing model, not a replica of real bank books.
