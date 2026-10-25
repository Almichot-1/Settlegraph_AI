# Next.js workstation in the integrated repository

The web app is in `settlegraph-web/`, alongside the Go services and `mobile_app/`. Read [the architecture](ARCHITECTURE.md), [the shared contract](../../contracts/settlegraph-public.openapi.yaml), [the generated operation manifest](../OPERATION_MANIFEST.json), [client handoff](../../FRONTEND_MOBILE_HANDOFF.md) and [local testing instructions](../local-testing.md). The public contract currently has 40 operations. Its generated SHA is recorded in `settlegraph-web/src/generated/contract-sha256.txt` and checked automatically.

From the repository root:

```sh
make bootstrap
make local-up
```

Open http://localhost:3000/login. The Docker web service uses the real Go API. Local owner sign-in has an explicit development shortcut; role-specific codes are in the ignored root `.env`. Access codes select server-owned identities; entering a different display name does not grant a role. Never copy credentials into `NEXT_PUBLIC_*` variables.

For host editing, follow the hot-reload instructions in the local runbook, then:

```sh
make frontend-install
make frontend-check frontend-test frontend-build
make frontend-dev
```

Mock mode remains available for visual work and is not runtime integration evidence. Keep snake_case fields, money strings, pagination, 202 polling, idempotency keys and quoted case versions compatible with the normative YAML. Propose changes in the shared contract first and regenerate both the types and manifest with `scripts/regenerate_openapi_types.py`.

Actual test results are recorded in [implementation progress](../implementation-progress.md). The coordinated browser/native command is `make test-cross-client`; it must complete before joint acceptance is claimed. AWS and native iOS have separate external verification gates.
