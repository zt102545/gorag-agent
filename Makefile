.PHONY: run build indexer test docker-up docker-down

APP_ENV ?= local

run:
	APP_ENV=$(APP_ENV) go run ./cmd/server

build:
	go build -o bin/server ./cmd/server
	go build -o bin/indexer ./cmd/indexer

indexer:
	go run ./cmd/indexer --file ./data/knowledge/qa.csv

test:
	go test ./...

docker-up:
	docker compose -f deployments/docker-compose.yml up -d

docker-down:
	docker compose -f deployments/docker-compose.yml down
