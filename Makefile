# This Makefile is used to build, test and deploy this project.
#
# Usage: make help
#

export VERSION ?= $(shell git rev-parse --short=7 HEAD)
export SERVICE = blastbeat-api
export ORG = dselans
export ARCH ?= $(shell uname -m)
export USER ?= $(shell whoami)

SHELL := /bin/bash
REPO="github.com/$(ORG)/$(SERVICE)"

GO = CGO_ENABLED=$(CGO_ENABLED) GOFLAGS=-mod=vendor go
CGO_ENABLED ?= 0
GO_BUILD_FLAGS = -ldflags "-X ${REPO}/config.Version=${VERSION}"

# Utility functions
check_defined = \
	$(strip $(foreach 1,$1, \
		$(call __check_defined,$1,$(strip $(value 2)))))
__check_defined = $(if $(value $1),, \
	$(error undefined '$1' variable: $2))

# Pattern #1 example: "example : description = Description for example target"
# Pattern #2 example: "### Example separator text
help: HELP_SCRIPT = \
	if (/^([a-zA-Z0-9-\.\/]+).*?: description\s*=\s*(.+)/) { \
		printf "\033[34m%-40s\033[0m %s\n", $$1, $$2 \
	} elsif(/^\#\#\#\s*(.+)/) { \
		printf "\033[33m>> %s\033[0m\n", $$1 \
	}

.PHONY: help
help:
	@perl -ne '$(HELP_SCRIPT)' $(MAKEFILE_LIST)

### Dev

.PHONY: run
run: description = Run blastbeat-api locally
run:
	$(GO) run `ls -1 *.go | grep -v _test.go`

### Build

.PHONY: build/linux-amd64
build/linux-amd64: description = Build service for linux-amd64
build/linux-amd64: build/clean
	GOOS=linux GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) -o ./build/$(SERVICE)-linux-amd64

.PHONY: build/linux-x86_64
build/linux-x86_64: description = Build service for linux-x86_64
build/linux-x86_64: build/clean
	GOOS=linux GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) -o ./build/$(SERVICE)-linux-amd64

.PHONY: build/linux-arm64
build/linux-arm64: description = Build service for linux-arm64
build/linux-arm64: build/clean
	GOOS=linux GOARCH=arm64 $(GO) build $(GO_BUILD_FLAGS) -o ./build/$(SERVICE)-linux-arm64

.PHONY: build/darwin-amd64
build/darwin-amd64: description = Build service for darwin-amd64
build/darwin-amd64: build/clean
	GOOS=darwin GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) -o ./build/$(SERVICE)-darwin-amd64

.PHONY: build/darwin-arm64
build/darwin-arm64: description = Build service for darwin-arm64
build/darwin-arm64: build/clean
	GOOS=darwin GOARCH=arm64 $(GO) build $(GO_BUILD_FLAGS) -o ./build/$(SERVICE)-darwin-arm64

.PHONY: build/clean
build/clean: description = Remove existing build artifacts
build/clean:
	$(RM) ./build/$(SERVICE)-*

### THIS NEEDS TO BE UPDATED TO DOCKERHUB

.PHONY: build/docker
build/docker: description = Build & push blastbeat-api image
build/docker:
	AWS_ECR_URL=$(AWS_ECR_URL) \
	VERSION=$(VERSION) \
	docker buildx build --push --platform=linux/arm64 --build-arg VERSION=$(VERSION) \
    -t $(AWS_ECR_URL)/$(SERVICE):$(VERSION) \
	-f ./Dockerfile .

### Deploy

### Test

.PHONY: test
test: description = Run tests
test:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	$(GO) test ./...

.PHONY: testv
testv: description = Run tests with verbose output
testv:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	$(GO) test ./... -v

.PHONY: test/coverage
test/coverage: description = Run Go unit tests
test/coverage: GOFLAGS=
test/coverage:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	$(GO) test ./... -coverprofile c.out

.PHONY: test/gocyclo
test/gocyclo: description = Run gocyclo complexity analysis (threshold > 20)
test/gocyclo:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	@if ! command -v gocyclo >/dev/null 2>&1; then \
		echo "Installing gocyclo..."; \
		GOFLAGS= go install github.com/fzipp/gocyclo/cmd/gocyclo@latest; \
	fi
	@echo "Running gocyclo analysis (threshold > 20)..."
	@GOCYCLO_BIN=$$(go env GOPATH)/bin/gocyclo; \
	if [ ! -f "$$GOCYCLO_BIN" ]; then \
		GOCYCLO_BIN=$$(go env GOBIN)/gocyclo; \
	fi; \
	if [ ! -f "$$GOCYCLO_BIN" ]; then \
		echo "Error: gocyclo binary not found after installation"; \
		exit 1; \
	fi; \
	REPORT=$$("$$GOCYCLO_BIN" -over 20 -top 25 . | grep -v '_test\|_mock\|mock_\|\.pb\.go\|proto\|vendor' || true); \
	if [ -n "$$REPORT" ]; then \
		echo "Complexity violations found (>20):"; \
		echo "$$REPORT"; \
		exit 1; \
	else \
		echo "No complexity violations found. All functions are under threshold 20."; \
	fi
