.PHONY: build
build:
	go build -v ./cmd/edh-stats
.PHONY: test
	go test -v -race --timeout 30s ./...
.PHONY: pi
pi:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc go build -o edh-stats ./cmd/edh-stats
.DEFAULT_GOAL := build