# Local integrated demo

Use the repository root containing `compose.yaml`, `settlegraph-web/`, and `mobile_app/`.

```sh
make bootstrap
make local-up
```

This starts the Go API/worker, BankSim, separate PostgreSQL databases, and the web application in Docker. It migrates and seeds the databases, preserves existing records, generates ignored local credentials, and configures the web server to call the actual API. Open http://localhost:3000/login. API readiness is at http://localhost:8080/health/ready.

If a port is already occupied, select free ports explicitly and use the same settings on subsequent commands:

```sh
WEB_PORT=3001 API_PORT=9080 BANKSIM_PORT=9082 make local-up
```

`make local-status` shows services; `make local-logs` shows recent logs. `make local-down` stops the integrated stack and retains database volumes. Starting it again preserves scenarios and audit history. Do not delete volumes to restart a demo.

For editing web code with hot reload on the host, stop the Compose web service first, install locked dependencies with `make frontend-install`, and supply an ignored `settlegraph-web/.env.local` containing backend mode, the host API URL, and the local demo access code. Then run `make frontend-dev`. Credentials belong only in local environment files and must not be committed.

```sh
make test lint
make test-integration
make contract-check
make test-contract-runtime
make test-e2e
make frontend-check frontend-test frontend-build
```

`make test-integration` creates one disposable PostgreSQL container, runs each Go integration test on a freshly migrated/seeded database, and removes only that container. It fails on skipped integration tests. `make test-e2e` runs synthetic scenarios and service restart recovery against the selected Compose project; run it when no manual reviewer is using that demo.

Mobile setup and native acceptance are tracked in the integration progress report. The shared host Flutter installation may differ from the pinned mobile SDK; do not assume a successful web build verifies the native app.
