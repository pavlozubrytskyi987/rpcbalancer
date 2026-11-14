.PHONY: run build test lint tidy

run:
	go run ./cmd/lb

build:
	mkdir -p bin
	go build -o bin/lb ./cmd/lb

test:
	go test -race ./...

lint:
	go vet ./...
	gofmt -l .

tidy:
	go mod tidy
