# This Makefile is used to build, test and deploy this project.
#
# Usage: make help
#
# NOTE: This Makefile relies heavily on scripts in the ./assets/scripts dir.
#

export VERSION ?= $(shell git rev-parse --short=7 HEAD)
export SERVICE = go-svc-template
export ORG = superpowerdotcom
export ARCH ?= $(shell uname -m)
export USER ?= $(shell whoami)

SHELL := /bin/bash
AWS_REGION ?= us-east-1
AWS_ACCOUNT_ID ?= $(shell command -v aws >/dev/null 2>&1 || { echo "ERROR: 'aws' CLI tool is not installed." >&2; exit 1; }; aws sts get-caller-identity --query Account --output text)
AWS_REGISTRY_ID ?= $(shell command -v aws >/dev/null 2>&1 || { echo "ERROR: 'aws' CLI tool is not installed." >&2; exit 1; }; aws ecr describe-registry --region us-east-1 --query registryId --output text)
AWS_ECR_URL ?= $(AWS_ACCOUNT_ID).dkr.ecr.us-east-1.amazonaws.com
STG_DEPLOYMENT_MSG = ":large_yellow_circle: *[STG]* Deployment :large_yellow_circle:"
PRD_DEPLOYMENT_MSG = ":large_green_circle: *[PRD]* Deployment :large_green_circle:"
SHARED_SCRIPT=./assets/scripts/shared.sh
DOPPLER_ENV ?= dev

GO = CGO_ENABLED=$(CGO_ENABLED) GOFLAGS=-mod=vendor go
CGO_ENABLED ?= 0
GO_BUILD_FLAGS = -ldflags "-X main.version=${VERSION}"

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
run: description = Run go-svc-template locally
run: prereq
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	$(GO) run `ls -1 *.go | grep -v _test.go`

.PHONY: run/docker
docker/run: description = Build and run container + deps via docker-compose
docker/run:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	docker-compose up -d

.PHONY: run/deps
run/deps: description = Run/start dependencies
run/deps:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	docker-compose -f ./docker-compose.yml up -d rabbitmq

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

.PHONY: build/docker
build/docker: description = Build & push go-svc-template image
build/docker: prereq util/login-aws-ecr
build/docker:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	AWS_ECR_URL=$(AWS_ECR_URL) \
	VERSION=$(VERSION) \
	docker buildx build --push --platform=linux/arm64,linux/amd64 --build-arg VERSION=$(VERSION) \
    -t $(AWS_ECR_URL)/$(SERVICE):$(VERSION) \
	-t $(AWS_ECR_URL)/$(SERVICE):latest \
	-f ./Dockerfile .

### Deploy

.PHONY: deploy/stg
deploy/stg: description = Deploy go-svc-template image to K8S
deploy/stg: DOPPLER_ENV=stg
deploy/stg: prereq check-doppler-secrets
	@bash $(SHARED_SCRIPT) info "Creating deployment notification in Slack ..."
	@TARGET=$@ bash $(SHARED_SCRIPT) notify $(STG_DEPLOYMENT_MSG)
	@bash $(SHARED_SCRIPT) info "Performing K8S deployment ..."
	aws eks update-kubeconfig --name staging-cluster --region $(AWS_REGION) && \
	doppler secrets substitute -p $(SERVICE) -c $(DOPPLER_ENV) deploy.$(DOPPLER_ENV).yml |  \
	sed "s/__VERSION__/$(VERSION)/g" | \
	sed "s/__SERVICE__/$(SERVICE)/g" | \
	sed "s/__AWS_ECR_URL__/$(AWS_ECR_URL)/g" | \
	kubectl apply -f -

.PHONY: deploy/prd
deploy/prd: description = Deploy go-svc-template image to K8S
deploy/prd: DOPPLER_ENV=prd
deploy/prd: prereq check-doppler-secrets
	@bash $(SHARED_SCRIPT) info "Creating deployment notification in Slack ..."
	@TARGET=$@ bash $(SHARED_SCRIPT) notify $(PRD_DEPLOYMENT_MSG)
	@bash $(SHARED_SCRIPT) info "Performing K8S deployment ..."
	aws eks update-kubeconfig --name production-cluster --region $(AWS_REGION) && \
	doppler secrets substitute -p $(SERVICE) -c $(DOPPLER_ENV) deploy.$(DOPPLER_ENV).yml | \
	sed "s/__VERSION__/$(VERSION)/g" | \
	sed "s/__SERVICE__/$(SERVICE)/g" | \
	sed "s/__AWS_ECR_URL__/$(AWS_ECR_URL)/g" | \
	kubectl apply -f -

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

### Utility

.PHONY: util/setup/shared
util/setup/shared:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	brew install jq && \
	brew install kubectl

.PHONY: util/setup/linux
util/setup/linux: description = Install dev tools for linux
util/setup/linux: util/setup/shared
	GO111MODULE=off go get github.com/maxbrunsfeld/counterfeiter

.PHONY: util/setup/darwin
util/setup/darwin: description = Install dev tools for darwin
util/setup/darwin: util/setup/shared
	GO111MODULE=off go get github.com/maxbrunsfeld/counterfeiter

.PHONY: util/login-aws-ecr
util/login-aws-ecr: description = Login to AWS ECR
util/login-aws-ecr:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	aws ecr get-login-password --region us-east-1 | \
	docker login --username AWS --password-stdin $(AWS_ECR_URL)

.PHONY: util/k8s/context/stg
util/k8s/context/stg: description = Set K8S context to staging cluster
util/k8s/context/stg:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	aws eks update-kubeconfig --name staging-cluster --region $(AWS_REGION)

.PHONY: util/k8s/context/prd
util/k8s/context/prd: description = Set K8S context to production cluster
util/k8s/context/prd:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	aws eks update-kubeconfig --name production-cluster --region $(AWS_REGION)

# ------------------- non-public targets --------------------

# Check if user is logged into Doppler
.PHONY: check-doppler-token
check-doppler-token:
	@bash $(SHARED_SCRIPT) info "Checking Doppler token ..."
	@if ! doppler configure get token > /dev/null 2>&1; then \
    	bash $(SHARED_SCRIPT) fatal "Doppler is not configured. Please log in using 'doppler login'"; \
 	fi

# Check if any secrets are missing (ie. have 'no-value') in Doppler (default DOPPLER_ENV to 'dev')
.PHONY: check-doppler-secrets
check-doppler-secrets:
	@bash $(SHARED_SCRIPT) info "Checking for missing secrets ..."
	@if doppler secrets substitute -p $(SERVICE) -c $(DOPPLER_ENV) $(DEPLOYMENT_DIR)/deploy.$(DOPPLER_ENV).yml | grep -B 1 "<no value>"; then \
		bash $(SHARED_SCRIPT) fatal "Found missing secret(s) in '$(DEPLOYMENT_DIR)/deploy.$(DOPPLER_ENV).yml'"; \
	fi

.PHONY: prereq
prereq:
	@bash $(SHARED_SCRIPT) info "Checking prerequisites ..."
	@bash $(SHARED_SCRIPT) prereq

.PHONY: debug/slack
debug/slack:
	@bash $(SHARED_SCRIPT) info "Sending a slack message ..."
	@TARGET=$@ bash $(SHARED_SCRIPT) notify "This is a test message"

.PHONY: debug/log
debug/log:
	@bash $(SHARED_SCRIPT) info "Installing tools ..."
