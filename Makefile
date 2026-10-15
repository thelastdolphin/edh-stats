BINARY := edh-stats
PKG := ./cmd/edh-stats
IMAGE := edh-stats

.DEFAULT_GOAL := build

.PHONY: build
build:
	go build -v -o $(BINARY) $(PKG)

.PHONY: run
run:
	go run $(PKG)

# Portable test command, and the one CI runs.
#
# The race detector is deliberately not part of it: ThreadSanitizer only supports
# certain virtual address layouts, and the arm64 deployment host is not one of
# them ("unsupported VMA range, Found 39 - Supported 48"), so every test binary
# dies there before running a test.
.PHONY: test
test:
	go test -p 1 -count=1 -timeout 60s ./...

# Race detector, for machines whose kernel ThreadSanitizer supports.
.PHONY: test-race
test-race:
	go test -count=1 -timeout 60s ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: check
check: vet test

# Cross-compiling needs a matching C toolchain, because the sqlite3 driver is a
# cgo package:
#   sudo apt-get install gcc-aarch64-linux-gnu libc6-dev-arm64-cross
.PHONY: build-arm64
build-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc go build -o $(BINARY) $(PKG)

# Kept as an alias of build-arm64 for muscle memory.
.PHONY: pi
pi: build-arm64

# The Dockerfile only assembles the image, so the binary has to exist first: these
# targets compile it for the matching architecture and then build the image.
.PHONY: image-arm64
image-arm64: build-arm64
	docker buildx build --platform linux/arm64 -t $(IMAGE):arm64 --load .

.PHONY: image-amd64
image-amd64: build
	docker buildx build --platform linux/amd64 -t $(IMAGE):amd64 --load .
