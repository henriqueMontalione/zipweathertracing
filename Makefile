.PHONY: up down logs build test test-coverage lint clean

up:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f

build:
	docker compose build

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
