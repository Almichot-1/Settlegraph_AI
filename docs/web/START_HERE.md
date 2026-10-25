# SETTLEGRAPH AI — NEXT.JS WEB WORKSTATION

**Send this whole ZIP to the WEB developer.** It includes the architecture blueprint, diagrams, working starter source, exact backend API contract, sample payloads, and implementation/review instructions.

## Read in this order

1. `ARCHITECTURE.pdf` — presentation-ready architecture document with diagrams.
2. `ARCHITECTURE.md` — authoritative editable engineering detail, same design as PDF.
3. `IMPLEMENTATION_PLAN.md` — dependency-ordered tickets with acceptance checks.
4. `contracts/settlegraph-public.openapi.yaml` — exact, shared public API contract; **do not edit silently**.
5. `docs/API_OPERATION_MATRIX.csv` — all 39 public API operations with MVP/P1/P2 flags.
6. `DEVELOPER_AGENT_PROMPT.md` — copy directly into the developer's coding assistant.
7. `starter/settlegraph-web/` — original runnable starter from the previously supplied handoff.

## Kickoff commands

```bash
cd starter/settlegraph-web
cp .env.example .env.local
npm install
npm run check:contracts
npm run typecheck
npm test
npm run dev
```
Open `http://localhost:3000`. Start in `SETTLEGRAPH_DATA_MODE=mock`; set `SETTLEGRAPH_API_BASE_URL` server-side only when Go endpoints exist.

## Operating rules

- Separate Git repository; never make modifications inside the autonomous Go backend's repository.
- Use the same OpenAPI SHA across all three teams: `13fa9283cb3ba51fbed8ed4b0ee9405090acc20b9fb58b0c3cc6368ee1cab83c`.
- Only the Go backend or its isolated BankSim adapter can determine financial truth. Clients **display** source observations, matching results and reviewed decisions.
- All data in mock mode and BankSim simulation is synthetic. No real bank balances or actual money movement are implied.
- Treat DTO schemas, required headers, enum values, success statuses and error shapes as contract, not suggestions.
- If a backend operation is missing, use mock mode and log a blocker. Never silently create an incompatible temporary endpoint.
- Make a Git commit and report exact command outputs after each accepted implementation ticket.

## Delivery proof you must send to the team

- Git revision plus branch
- Screenshots or short screen recording of delayed settlement scenario
- Test/build outputs and outstanding failures
- OpenAPI contract SHA / changes requested
- Back-end integration status, distinguishing mock-tested from live-tested
- Security review and unresolved risks

**Note:** A starter package is not evidence that full runtime or device tests have passed. The backend is under independent development; coordinate integration through the shared public API.
