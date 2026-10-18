BINARY := edh-stats
PKG := ./cmd/edh-stats
IMAGE := edh-stats

# Quality tools are pinned: a version that moves on its own is a build that turns
# red on a Tuesday for no reason. Both are fetched on demand, never installed
# globally and never into the image.
GOLANGCI_LINT_VERSION := v2.15.0
GOVULNCHECK_VERSION := v1.9.0
TOOLS_DIR := .tools

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

# Static analysis. The linter is the official release binary for this
# architecture: building golangci-lint from source takes minutes, and on the
# four-core deployment host it is exactly the cold, parallel compile that machine
# has failed at before.
.PHONY: lint
lint:
	@set -e; \
	version=$(GOLANGCI_LINT_VERSION); bare=$${version#v}; \
	arch=$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'); \
	bin=$(TOOLS_DIR)/golangci-lint-$$bare; \
	if [ ! -x "$$bin" ]; then \
		echo "downloading golangci-lint $$version for $$arch"; \
		mkdir -p $(TOOLS_DIR); \
		curl -sSfL -o $(TOOLS_DIR)/lint.tgz \
			"https://github.com/golangci/golangci-lint/releases/download/$$version/golangci-lint-$$bare-linux-$$arch.tar.gz"; \
		tar -xzf $(TOOLS_DIR)/lint.tgz -C $(TOOLS_DIR); \
		mv $(TOOLS_DIR)/golangci-lint-$$bare-linux-$$arch/golangci-lint "$$bin"; \
		rm -rf $(TOOLS_DIR)/lint.tgz $(TOOLS_DIR)/golangci-lint-$$bare-linux-$$arch; \
	fi; \
	"$$bin" run --concurrency 2

# Vulnerabilities in the dependencies the code actually calls. It needs the network
# because the database lives at vuln.go.dev, so a new advisory can turn this red
# without a commit - which is the point of a security gate.
.PHONY: vulncheck
vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

# End-to-end: builds the real binary, starts it as a process and talks to it over
# HTTP. Behind the e2e build tag, so `make test` stays fast and the end-to-end test
# never runs by accident.
.PHONY: e2e
e2e:
	go vet -tags e2e ./...
	go test -tags e2e -count=1 -timeout 120s ./e2e/
