# Makefile

.PHONY: hllo build run deps fmt vendor all

hllo:
	echo "Hello"

deps:
	go mod tidy

fmt:
	go fmt ./...

vendor:
	go mod vendor

build: fmt deps vendor
	go build -o bin/gsched cmd/main.go

run: fmt deps vendor
	go run cmd/main.go

all: deps fmt vendor build