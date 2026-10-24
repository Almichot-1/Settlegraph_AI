# Active risk log

The authoritative starting risk list is in `PROJECT_MANAGER_DELIVERY_PLAYBOOK.md`, section 8. Update this file with **observed** changes only. Do not premark mitigations complete.

| Risk ID | Current assessment | Owner | Evidence / next mitigation | State |
|---|---|---|---|---|
| R-01 Solo backend scope overload | HIGH | Project lead | Reassess at each gate; protect first E2E slice | OPEN |
| R-02 Bedrock account/model unavailable | MEDIUM | Backend | Verify access; preserve labeled local stub | OPEN |
| R-04 Ledger concurrency correctness | MEDIUM residual | Backend | Isolated PostgreSQL test passed: 100 same-key retries and 100 distinct attempts; repeat after ledger lifecycle changes | MITIGATED_LOCAL |
| R-06 OpenAPI breaking changes | HIGH | Project lead | Contract freeze + client acknowledgment | OPEN |
| R-09 Live demo network instability | MEDIUM | Project lead | Local compose recovery rehearsal | OPEN |
| R-10 Local trust authentication copied to deployment | HIGH | Backend | Compose DB ports are unpublished and DB networks private; replace trust with secret-managed authentication before any deployment | OPEN |
| R-11 Provider delivery failure leaves source incomplete | MEDIUM residual | Backend | Local fault injection passed: BankSim redelivery, deduplicated inbox, coverage/cursor repair and reconciliation after restart; retest under cloud queue configuration | MITIGATED_LOCAL |
| R-12 Unfinished public import routes | MEDIUM | Backend | Three optional P2 import routes are not registered; do not promise CSV/S3 fallback to clients yet | OPEN |

Create a new row immediately for any newly discovered blocker, severity, owner, and reproducible evidence.
