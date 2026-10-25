/** GENERATED FROM ../../contracts/settlegraph-public.openapi.yaml. DO NOT EDIT BY HAND. */
/** The authoritative schema is OpenAPI 3.0.3, version 2.0.0-draft. */
export type Member = {
  "id": string;
  "display_name": string;
  "role": "OWNER" | "ANALYST" | "REVIEWER" | "VIEWER";
};

export type MemberPage = {
  "items": Array<Member>;
  "next_cursor": (string) | null;
};

export type Money = {
  "minor_units": string;
  "currency": "ETB";
};

export type ApiError = {
  "error": {
  "code": string;
  "message": string;
  "request_id": string;
  "details"?: Record<string, unknown>;
};
};

export type Role = "OWNER" | "ANALYST" | "REVIEWER" | "VIEWER";

export type Organization = {
  "id": string;
  "name": string;
  "mode": "DEMO" | "PRODUCTION";
};

export type Membership = {
  "organization": Organization;
  "role": Role;
};

export type UserProfile = {
  "id": string;
  "display_name": string;
  "memberships": Array<Membership>;
};

export type DemoSessionRequest = {
  "display_name": string;
};

export type DemoSessionResponse = {
  "access_token": string;
  "token_type": "Bearer";
  "expires_at": string;
  "user": UserProfile;
  "default_organization_id": string;
};

export type HealthStatus = {
  "status": "UP" | "NOT_READY";
  "service": string;
};

export type Provider = {
  "id": string;
  "name": string;
  "kind": "BANK" | "WALLET" | "GATEWAY";
  "sync_status": "HEALTHY" | "STALE" | "OUTAGE" | "NEVER_SYNCED";
  "last_observed_at": (string) | null;
};

export type ProviderList = {
  "items": Array<Provider>;
  "next_cursor": (string) | null;
};

export type DashboardSummary = {
  "as_of": string;
  "orders_expected": Money;
  "payments_accepted": Money;
  "settlements_observed": Money;
  "unmatched_payments": Money;
  "open_cases": number;
  "providers": Array<Provider>;
  "coverage": {
  "source_status": "COMPLETE" | "INCOMPLETE_SOURCE" | "STALE";
  "is_complete": boolean;
};
};

export type TimePoint = {
  "bucket_start": string;
  "payments_accepted": Money;
  "settlements_observed": Money;
};

export type DashboardSeries = {
  "granularity": "HOUR" | "DAY";
  "points": Array<TimePoint>;
};

export type AccountView = {
  "id": string;
  "provider_id": string;
  "account_label": string;
  "account_type": "MERCHANT" | "CUSTOMER";
  "last_observed_at": (string) | null;
  "observed_balance"?: Money;
};

export type AccountPage = {
  "items": Array<AccountView>;
  "next_cursor": (string) | null;
};

export type Order = {
  "id": string;
  "external_reference": string;
  "amount": Money;
  "status": "OPEN" | "PAYMENT_OBSERVED" | "RECONCILED";
  "created_at": string;
  "scenario_run_id"?: (string) | null;
};

export type OrderPage = {
  "items": Array<Order>;
  "next_cursor": (string) | null;
};

export type Transaction = {
  "id": string;
  "provider_id": string;
  "external_reference": string;
  "amount": Money;
  "state": "ACCEPTED" | "FAILED" | "REFUNDED";
  "occurred_at": string;
  "observed_at": string;
  "order_reference"?: (string) | null;
  "source_attestation": "SIGNED_SIMULATED_PROVIDER" | "IMPORTED_UNVERIFIED";
  "scenario_run_id"?: (string) | null;
};

export type TransactionPage = {
  "items": Array<Transaction>;
  "next_cursor": (string) | null;
};

export type Settlement = {
  "id": string;
  "provider_id": string;
  "external_reference": string;
  "payment_reference": string;
  "amount": Money;
  "occurred_at": string;
  "observed_at": string;
  "scenario_run_id"?: (string) | null;
};

export type SettlementPage = {
  "items": Array<Settlement>;
  "next_cursor": (string) | null;
};

export type CreateRunRequest = {
  "scope": "LATEST_OBSERVATIONS" | "SCENARIO_RUN";
  "scenario_run_id"?: (string) | null;
  "rules_version"?: string;
};

export type ReconciliationRun = {
  "id": string;
  "status": "QUEUED" | "RUNNING" | "COMPLETED" | "FAILED";
  "scope": "LATEST_OBSERVATIONS" | "SCENARIO_RUN";
  "rules_version": string;
  "created_at": string;
  "completed_at"?: (string) | null;
  "matched_order_payment": number;
  "matched_payment_settlement": number;
  "unmatched_count": number;
  "scenario_run_id"?: (string) | null;
};

export type RunPage = {
  "items": Array<ReconciliationRun>;
  "next_cursor": (string) | null;
};

export type Match = {
  "id": string;
  "run_id": string;
  "stage": "ORDER_PAYMENT" | "PAYMENT_SETTLEMENT";
  "left_evidence_id": string;
  "right_evidence_id": string;
  "allocated_amount": Money;
  "rule": "EXACT_PROVIDER_REFERENCE_AND_AMOUNT" | "EXACT_ORDER_REFERENCE_AND_AMOUNT";
  "proof_id": string;
};

export type MatchPage = {
  "items": Array<Match>;
  "next_cursor": (string) | null;
};

export type MatchProof = {
  "id": string;
  "run_id": string;
  "rule_version": string;
  "stage": "ORDER_PAYMENT" | "PAYMENT_SETTLEMENT";
  "checks": Array<string>;
  "source_evidence_ids": Array<string>;
  "result": "MATCHED_IN_IMPORTED_RECORDS";
};

export type Evidence = {
  "id": string;
  "record_type": "ORDER" | "PAYMENT" | "SETTLEMENT" | "SOURCE_STATUS";
  "reference": string;
  "amount": (Money) | null;
  "occurred_at": string;
  "provider_id": (string) | null;
  "raw_payload_sha256"?: (string) | null;
};

export type EvidencePage = {
  "items": Array<Evidence>;
  "next_cursor": (string) | null;
};

export type Case = {
  "id": string;
  "kind": "SETTLEMENT_NOT_OBSERVED" | "AMOUNT_MISMATCH" | "AMBIGUOUS_MATCH" | "SOURCE_UNAVAILABLE";
  "status": "OPEN" | "EVIDENCE_UPDATED" | "RESOLVED";
  "severity": "LOW" | "MEDIUM" | "HIGH";
  "title": string;
  "summary": string;
  "version": number;
  "created_at": string;
  "updated_at": string;
  "evidence_count": number;
  "assigned_to"?: (string) | null;
  "scenario_run_id"?: (string) | null;
};

export type CasePage = {
  "items": Array<Case>;
  "next_cursor": (string) | null;
};

export type CaseEvent = {
  "id": string;
  "case_id": string;
  "type": "OPENED" | "EVIDENCE_ADDED" | "INVESTIGATION_REQUESTED" | "INVESTIGATION_COMPLETED" | "ASSIGNED" | "DECISION_RECORDED" | "RESOLVED";
  "occurred_at": string;
  "actor": string;
  "message": string;
  "version": number;
};

export type CaseEventPage = {
  "items": Array<CaseEvent>;
  "next_cursor": (string) | null;
};

export type AssignCaseRequest = {
  "assignee_id": string;
};

export type ReviewDecisionRequest = {
  "decision": "RESOLVE" | "REOPEN" | "NEEDS_MORE_EVIDENCE";
  "reason": string;
};

export type InvestigationRequest = {
  "mode": "EVIDENCE_ANALYSIS";
};

export type Investigation = {
  "id": string;
  "case_id": string;
  "status": "QUEUED" | "RUNNING" | "COMPLETED" | "REJECTED" | "FAILED";
  "provider": "BEDROCK" | "OFFLINE_STUB";
  "created_at": string;
  "evidence_snapshot_hash": string;
  "completed_at"?: (string) | null;
  "result"?: InvestigationResult;
};

export type InvestigationResult = {
  "summary": string;
  "hypotheses": Array<{
  "explanation": string;
  "evidence_ids": Array<string>;
}>;
  "recommended_actions": Array<string>;
  "evidence_ids": Array<string>;
  "limitations": Array<string>;
};

export type InvestigationPage = {
  "items": Array<Investigation>;
  "next_cursor": (string) | null;
};

export type Job = {
  "id": string;
  "kind": "RECONCILE" | "INVESTIGATE" | "IMPORT" | "BACKFILL";
  "status": "QUEUED" | "RUNNING" | "COMPLETED" | "FAILED";
  "created_at": string;
  "finished_at"?: (string) | null;
  "error_code"?: (string) | null;
};

export type AuditEvent = {
  "id": string;
  "actor": string;
  "action": string;
  "resource_type": string;
  "resource_id": string;
  "occurred_at": string;
};

export type AuditEventPage = {
  "items": Array<AuditEvent>;
  "next_cursor": (string) | null;
};

export type ScenarioDefinition = {
  "key": "normal_flow" | "delayed_settlement" | "duplicate_webhook" | "provider_outage";
  "title": string;
  "description": string;
  "steps": number;
};

export type ScenarioList = {
  "items": Array<ScenarioDefinition>;
};

export type StartScenarioRequest = {
  "seed": number;
};

export type ScenarioRun = {
  "id": string;
  "scenario_key": "normal_flow" | "delayed_settlement" | "duplicate_webhook" | "provider_outage";
  "status": "PREPARING" | "READY" | "RUNNING" | "COMPLETED" | "FAILED";
  "step": number;
  "total_steps": number;
  "virtual_now": string;
  "organization_id": string;
  "created_at": string;
};

export type AdvanceScenarioRequest = {
  "expected_step": number;
};

export type WebhookAccepted = {
  "accepted": boolean;
  "duplicate": boolean;
};

export type Import = {
  "id": string;
  "status": "AWAITING_UPLOAD" | "QUEUED" | "PROCESSING" | "COMPLETED" | "FAILED";
  "filename": string;
  "source_kind": "ORDERS" | "PAYMENTS" | "SETTLEMENTS";
  "created_at": string;
  "accepted_records": number;
  "rejected_records": number;
};

export type ImportPage = {
  "items": Array<Import>;
  "next_cursor": (string) | null;
};

export type BankEvent = {
  "event_id": string;
  "provider_id": string;
  "scenario_run_id": string;
  "sequence": number;
  "event_type": "PAYMENT_ACCEPTED" | "SETTLEMENT_POSTED" | "REFUND_POSTED" | "TRANSFER_FAILED";
  "occurred_at": string;
  "transfer_reference": string;
  "amount": Money;
  "payload_version": 1;
  "order_reference"?: (string) | null;
  "payment_reference"?: (string) | null;
};

export type CreateImportRequest = {
  "filename": string;
  "source_kind": "ORDERS" | "PAYMENTS" | "SETTLEMENTS";
  "content_sha256": string;
  "file_size_bytes": number;
};

export type ImportUploadSession = {
  "import": Import;
  "upload_url": string;
  "upload_headers": Record<string, string>;
  "expires_at": string;
};

export type CompleteImportRequest = {
  "content_sha256": string;
};
