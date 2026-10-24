.PHONY: build lint test test-integration test-e2e contract-check migrate seed up down smoke init-demo-env

init-demo-env:
	python validation/init_demo_env.py

build:
	mkdir -p bin
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker
	go build -o bin/banksim ./cmd/banksim

lint:
	go vet ./...

test:
	go test ./...

test-integration:
	go test -tags integration ./...

test-e2e:
	python tests/e2e_demo.py

contract-check:
	python validation/validate_pack.py

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
