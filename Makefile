.PHONY: build run-api run-bot run-all test up down logs vet

build:
	go build -o family-expenses .

run-api:
	go run . -service api

run-bot:
	go run . -service bot

run-all:
	go run . -service all

test:
	go test ./...

vet:
	go vet ./...

up:
	cd deployments && docker compose up -d --build

down:
	cd deployments && docker compose down

logs:
	cd deployments && docker compose logs -f