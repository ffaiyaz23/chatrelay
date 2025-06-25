.PHONY: build run test lint docker compose

build:
	go build -o bin/chatrelay ./cmd/chatrelay

run:
	go run ./cmd/chatrelay

test:
	go test ./... -cover

lint:
	golangci-lint run

docker:
	docker build -t chatrelay .

compose:
	docker compose up --build
