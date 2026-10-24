# Active risk log

The authoritative starting risk list is in `PROJECT_MANAGER_DELIVERY_PLAYBOOK.md`, section 8. Update this file with **observed** changes only. Do not premark mitigations complete.

| Risk ID | Current assessment | Owner | Evidence / next mitigation | State |
|---|---|---|---|---|
| R-01 Solo backend scope overload | HIGH | Project lead | Reassess at each gate; protect first E2E slice | OPEN |
| R-02 Bedrock account/model unavailable | MEDIUM | Backend | Verify access; preserve labeled local stub | OPEN |
| R-04 Ledger concurrency correctness | CRITICAL if defect exists | Backend | Must execute concurrent transfer tests | NOT_TESTED |
| R-06 OpenAPI breaking changes | HIGH | Project lead | Contract freeze + client acknowledgment | OPEN |
| R-09 Live demo network instability | MEDIUM | Project lead | Local compose recovery rehearsal | OPEN |

Create a new row immediately for any newly discovered blocker, severity, owner, and reproducible evidence.
