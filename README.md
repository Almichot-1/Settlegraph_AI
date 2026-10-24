# SettleGraph AI — Solo Backend Execution Pack

**Prepared:** 24 October 2026 (v2.1 PM extension)  
**Status:** Design specification + static contracts + synthetic fixtures; **not an implemented application**.  
**Purpose:** Give one Go/backend implementation agent a phased build plan while Next.js and Flutter engineers build against stable HTTP contracts.

## Open these files in this order

1. `PROJECT_MANAGER_DELIVERY_PLAYBOOK.md` — project charter, 22 implementation tickets, dependencies, realistic staffing, risk register and final acceptance.
2. `BACKEND_IMPLEMENTATION_BLUEPRINT.md` — authoritative implementation phases, detailed checkboxes, invariants, acceptance tests and stopping rules.
3. `contracts/settlegraph-public.openapi.yaml` — **normative web + Flutter API** (39 endpoints). Freeze property names and enum values before UI integration.
4. `contracts/banksim-internal.openapi.yaml` — BankSim provider/admin/evaluator boundaries (10 endpoints).
5. `contracts/bank-event.schema.json` — incoming signed synthetic provider event body.
6. `contracts/ai-investigation.schema.json` — bounded Bedrock investigation response structure.
7. `FRONTEND_MOBILE_HANDOFF.md` — exact web/Flutter endpoints, mock-data handling, authentication and polling.
8. `AGENT_IMPLEMENTATION_INSTRUCTIONS.md` — prompt you can paste directly into a coding agent.
9. `db/001_settlegraph.sql` and `db/001_banksim.sql` — **starter** migrations for separate databases.
10. `fixtures/` — deterministic scenarios, example request/response JSON, and adversarial AI sample.
11. `docs/` — editable kickoff ADR, API change log, progress reporting and risk register.
12. `AGENT_BACKLOG.json` — machine-readable dependency graph for the 22 SG-### tickets.
13. `validation/` — offline generators and a static validation script.

## What we are building

A **simulated** two-bank payment ecosystem (`BankSim`), and an independently deployed evidence-backed financial reconciliation product (`SettleGraph`). The Go backend runs as three binaries: API, worker and BankSim. The product uses a separate PostgreSQL DB and must not read simulator private financial state. An accepted payment and a bank settlement are different observations. Bedrock suggests explanations **but never creates or approves payment facts**.

## Build checkpoints

| Checkpoint | Required evidence |
|---|---|
| Gate 0 | Contracts, mock fixtures, three Go binaries compile |
| Gate 1 | Demo auth, org isolation, structured errors |
| Gate 2 | Balanced BankSim ledger and deterministic transfers |
| Gate 3 | HMAC webhook and durable deduplicated observations |
| Gate 4 | Deterministic two-stage reconciliation with proof |
| Gate 5 | Dashboard/case reads, frontend/mobile working |
| Gate 6 | Bedrock (or labeled offline stub), review and audit |
| Gate 7 | Four fault/replay scenarios and case lifecycle |
| Gate 8 | Optional AWS deployment and operational smoke tests |

## Commands to validate **this specification pack**

From the pack directory:

```bash
python validation/validate_pack.py
```

Requires PyYAML and jsonschema. `validation/build_contracts.py` generates the YAML and schema source; `validation/generate_fixtures.py` regenerates synthetic fixtures. **Do not run generators after UI contract freeze unless you intend to change the corresponding YAML and mocks**. The validation script checks schema references, endpoint inventory, JSON samples and AI evidence mismatch fixtures. It does **not** verify that Go code or SQL migrations execute because backend code does not exist in this pack.

## Key conventions

- All currency is ETB; `minor_units` is a **decimal string** at the HTTP boundary, signed Go `int64` internally.
- All timestamps are RFC3339 UTC.
- All tenant endpoints need Bearer JWT and `X-Organization-Id` membership verification.
- Case mutations also need `Idempotency-Key` and `If-Match: "<version>"`.
- Webhook requests use per-provider HMAC on timestamp, event ID and raw bytes.
- Keep provider `complete_through` watermarks: **absence is not evidence if the provider's source window is incomplete**.
- Public demo tokens and simulated institutions must never be represented as a live bank connection.

## Primary demo

A customer attempts a simulated 2,500 ETB payment, BankSim accepts it and delays settlement. SettleGraph observes the payment, computes the incomplete two-stage match, opens a case after grace period and a complete source watermark, and uses Bedrock to generate an evidence-backed hypothesis. The simulated bank later settles, triggering a second reconciliation run and a visible case timeline update. Flutter reviewer records a human decision.

**No real bank integration, real account funds or real payment operation is included.**

## Project-manager operational control (v2.1 addition)

Read `PROJECT_MANAGER_DELIVERY_PLAYBOOK.md` **after** the architecture blueprint. It adds the scope charter, critical-path gates, stable SG-001 through SG-022 tickets, two delivery schedules (48-hour compressed and 10-day preparation), RACI/owner handoffs, quantified risk triggers, acceptance checklist, status reporting template and 5-minute judge demo. The plan does **not** claim backend code or real bank integrations exist.

`AGENT_BACKLOG.json` is a machine-readable, dependency-checked list of the 22 SG-001–SG-022 tickets. Its initial statuses are all `NOT_STARTED`; do not mistake it for implementation progress. Regenerate with `python validation/build_agent_backlog.py`.


## Autonomous execution (optional)

See `AUTONOMOUS_SETUP.md` and use `autonomous_build.py` to run the SG tickets through build/review/test/commit without manually typing continue. This is an opt-in local controller. Full-access `--unrestricted` bypasses Codex security boundaries and is dangerous on personal machines.
