.PHONY: build
build:
	go build -v ./cmd/edh-stats
.PHONY: test
	go test -v -race --timeout 30s ./...
.DEFAULT_GOAL := build