.PHONY: ci ci-jr ci-crm lint-install lint-jr lint-crm test-jr test-crm build-jr build-crm

ROOT_DIR := $(CURDIR)
GOBIN ?= $(ROOT_DIR)/bin
GOLANGCI_LINT_VERSION ?= v1.64.8
GOLANGCI_LINT := $(GOBIN)/golangci-lint

ci: ci-jr ci-crm

ci-jr: lint-jr test-jr build-jr

ci-crm: lint-crm test-crm build-crm

lint-install:
	mkdir -p "$(GOBIN)"
	@if [ ! -x "$(GOLANGCI_LINT)" ] || ! "$(GOLANGCI_LINT)" version 2>/dev/null | grep -q "version $(GOLANGCI_LINT_VERSION)"; then \
		GOBIN="$(GOBIN)" go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi

lint-jr: lint-install
	cd bronivik_jr && "$(GOLANGCI_LINT)" run --timeout=5m ./...

lint-crm: lint-install
	cd bronivik_crm && "$(GOLANGCI_LINT)" run --timeout=5m ./...

test-jr:
	cd bronivik_jr && go test -race ./...

test-crm:
	cd bronivik_crm && go test -race ./...

build-jr:
	cd bronivik_jr && tmpdir="$$(mktemp -d)" && \
		go build -o "$$tmpdir/bot" ./cmd/bot && \
		go build -o "$$tmpdir/api" ./cmd/api && \
		rm -rf "$$tmpdir"

build-crm:
	cd bronivik_crm && tmpdir="$$(mktemp -d)" && \
		go build -o "$$tmpdir/bot" ./cmd/bot && \
		rm -rf "$$tmpdir"
