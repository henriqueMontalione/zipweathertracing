.PHONY: up build down logs run-gateway run-orchestrator test test-coverage lint clean

up:
	docker compose up

build:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f

run-gateway:
	export $$(cat .env | xargs) && cd gateway && go run ./cmd/server

run-orchestrator:
	export $$(cat .env | xargs) && cd orchestrator && go run ./cmd/server

test:
	cd gateway && go test -race ./...
	cd orchestrator && go test -race ./...

test-coverage:
	cd gateway && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
	cd orchestrator && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

lint:
	cd gateway && go vet ./...
	cd orchestrator && go vet ./...

clean:
	rm -f gateway/coverage.out orchestrator/coverage.out
