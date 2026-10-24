-- Account views contain provider metadata only; no BankSim private balances.
CREATE TABLE business_accounts (
 organization_id uuid NOT NULL REFERENCES organizations(id),
 id uuid NOT NULL,
 provider_id text NOT NULL,
 account_label text NOT NULL,
 account_type text NOT NULL CHECK(account_type IN ('MERCHANT','CUSTOMER')),
 last_observed_at timestamptz,
 observed_balance_minor bigint,
 currency char(3) NOT NULL DEFAULT 'ETB' CHECK(currency='ETB'),
 PRIMARY KEY(organization_id,id),
 FOREIGN KEY(organization_id,provider_id) REFERENCES provider_sources(organization_id,id)
);
CREATE INDEX business_accounts_by_provider ON business_accounts(organization_id,provider_id,id);
