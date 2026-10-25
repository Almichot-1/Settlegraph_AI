.PHONY: build lint test test-integration test-e2e test-contract-runtime contract-check migrate seed up down smoke init-demo-env frontend-install frontend-dev frontend-test frontend-check frontend-build

PYTHON := .venv/bin/python
GO_PACKAGES := ./cmd/... ./internal/...

bootstrap:
	python3 -m venv .venv
	.venv/bin/pip install -q -r validation/requirements.txt

init-demo-env:
	python3 validation/init_demo_env.py

build:
	mkdir -p bin
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker
	go build -o bin/banksim ./cmd/banksim

lint:
	go vet $(GO_PACKAGES)

test:
	go test $(GO_PACKAGES)

test-integration:
	$(PYTHON) scripts/test_integration.py

test-e2e:
	$(PYTHON) tests/e2e_demo.py
	$(PYTHON) tests/e2e_delivery_recovery.py

test-contract-runtime:
	.venv/bin/python tests/contract_runtime.py

contract-check:
	$(PYTHON) validation/validate_pack.py

migrate:
	docker compose exec -T product-db psql -v ON_ERROR_STOP=1 -U settlegraph -d settlegraph -f /migrations/apply_settlegraph.sql
	docker compose exec -T banksim-db psql -v ON_ERROR_STOP=1 -U banksim -d banksim -f /migrations/apply_banksim.sql

seed:
	docker compose exec -T product-db psql -v ON_ERROR_STOP=1 -U settlegraph -d settlegraph -f /migrations/seed_settlegraph.sql
	docker compose exec -T banksim-db psql -v ON_ERROR_STOP=1 -U banksim -d banksim -f /migrations/seed_banksim.sql

up: init-demo-env
	docker compose up -d --build --wait

down:
	docker compose down

smoke:
	curl --fail --silent http://127.0.0.1:8080/health/live
	curl --fail --silent http://127.0.0.1:8080/health/ready
	docker compose exec -T worker wget -q -O /dev/null http://127.0.0.1:8081/health/ready
	curl --fail --silent http://127.0.0.1:8082/health/live
	curl --fail --silent http://127.0.0.1:8082/health/ready


frontend-install:
	cd settlegraph-web && npm ci

frontend-dev:
	cd settlegraph-web && npm run dev

frontend-test:
	cd settlegraph-web && npm test

frontend-check:
	cd settlegraph-web && npm run check:contracts && npm run typecheck

frontend-build:
	cd settlegraph-web && npm run build

.PHONY: bootstrap local-up local-down local-status local-logs
local-up:
	python3 scripts/local_stack.py up
local-down:
	python3 scripts/local_stack.py down
local-status:
	python3 scripts/local_stack.py status
local-logs:
	python3 scripts/local_stack.py logs
