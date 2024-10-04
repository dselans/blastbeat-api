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
FORWARD_SCRIPT=./assets/scripts/forward.sh
PROTOSET_SCRIPT=./assets/scripts/sync-protoset.sh
DEPLOY_SCRIPT=./assets/scripts/deploy.py
KSP_SCRIPT=./assets/scripts/ksp.sh
DOPPLER_CONFIG ?= dev
PLUMBER_QUEUE_NAME ?= plumber-$(shell date | sha256sum | cut -b 1-6)
PLUMBER_RABBITMQ_URL ?= amqp://localhost

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
run: description = Run go-svc-template locally + port-forward deps to localhost (ctrl-c to stop)
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
run/deps: prereq check-doppler-secrets util/dev/start
run/deps:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	kubectl config use-context minikube && \
	kubectl apply -f ./deploy.dev.yml

.PHONY: run/deps/forward
run/deps/forward: description = Forward ports for dependencies
run/deps/forward: prereq
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	@bash $(SHARED_SCRIPT) debug "Forwarding ports to minikube (ctrl-c to stop) ..."
	@bash $(FORWARD_SCRIPT) all

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
	docker buildx build --push --platform=linux/arm64 --build-arg VERSION=$(VERSION) \
    -t $(AWS_ECR_URL)/$(SERVICE):$(VERSION) \
	-f ./Dockerfile .

### Deploy

.PHONY: deploy/stg
deploy/stg: description = Deploy go-svc-template to staging (STG)
deploy/stg: prereq
	K8S_CLUSTER=staging-cluster \
	DOPPLER_PROJECT=go-svc-template \
	DOPPLER_CONFIG=stg \
	DEPLOY_ENV=STG \
	DEPLOY_CONFIG=deploy.stg.yml \
	KSP_SERVICE=go-svc-template \
	python3 $(DEPLOY_SCRIPT) -r go-svc-template -t deploy/hidden

.PHONY: deploy/prd
deploy/prd: description = Deploy go-svc-template to production (PRD)
deploy/prd: prereq check-doppler-secrets
	K8S_CLUSTER=production-cluster \
	DOPPLER_PROJECT=go-svc-template \
	DOPPLER_CONFIG=prd \
	DEPLOY_ENV=PRD \
	DEPLOY_CONFIG=deploy.prd.yml \
	KSP_SERVICE=go-svc-template \
	python3 $(DEPLOY_SCRIPT) -r go-svc-template -t deploy/hidden -f prd

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

.PHONY: util/dev/start
util/dev/start: description = Start minikube for local dev
util/dev/start:
	minikube status || minikube start

.PHONY: util/dev/stop
util/dev/stop: description = Stop minikube for local dev
util/dev/stop:
	minikube status || minikube stop

.PHONY: util/dev/reset
util/dev/reset: description = Reset minikube for local dev (delete all imgs, pods, etc)
util/dev/reset:
	minikube status || minikube delete

.PHONY: util/dev/forward
util/dev/forward: description = Forward ports to minikube for local dev
util/dev/forward:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	@bash $(SHARED_SCRIPT) debug "Forwarding ports to minikube (ctrl-c to stop) ..."
	@bash $(FORWARD_SCRIPT)

.PHONY: util/k8s/context/dev
util/k8s/context/dev: description = Set K8S context to local minikube
util/k8s/context/dev:
	@bash $(SHARED_SCRIPT) info "Running $@ ..."
	kubectl config use-context minikube

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

### Events

.PHONY: events/read
events/read: description = Continuously read events from bus in minikube
events/read: events/protoset
	@bash $(SHARED_SCRIPT) debug "Continuously reading events from bus ..."
	plumber read rabbit -f --pretty \
    --protobuf-descriptor-set ./assets/events/events.protoset \
    --protobuf-root-message common.Event \
    --address $(PLUMBER_RABBITMQ_URL) \
    --exchange-name events \
    --queue-name $(PLUMBER_QUEUE_NAME) \
    --queue-declare \
    --queue-delete \
    --binding-key \# \
    --decode-type protobuf

.PHONY: events/write/user-updated
events/write/user-updated: description = Emit user-updated event on bus in minikube
events/write/user-updated: events/protoset
	@bash $(SHARED_SCRIPT) debug "Writing user-updated event to bus ..."
	plumber write rabbit \
    --protobuf-descriptor-set ./assets/events/events.protoset \
    --protobuf-root-message common.Event \
    --address $(PLUMBER_RABBITMQ_URL) \
    --exchange-name events \
    --routing-key user.updated \
    --encode-type jsonpb \
    --input-file ./assets/events/user-updated.json

.PHONY: events/protoset
events/protoset: description = Sync events.protoset with events version specified in go.mod
events/protoset: prereq
	@bash $(SHARED_SCRIPT) debug "Syncing events.protoset with events version specified in go.mod ..."
	@doppler run -p shared -c prd --only-secrets GITHUB_TOKEN -- sh $(PROTOSET_SCRIPT)

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
	@bash $(SHARED_SCRIPT) info "Checking for missing secrets in $(DEPLOY_CONFIG) ..."
	@if doppler secrets substitute -p $(SERVICE) -c $(DOPPLER_CONFIG) $(DEPLOY_CONFIG) | grep -B 1 "<no value>"; then \
		bash $(SHARED_SCRIPT) fatal "Found missing secret(s) in '$(DEPLOY_CONFIG)'"; \
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

.PHONY: deploy/hidden
deploy/hidden: prereq check-doppler-secrets
	$(call check_defined, K8S_CLUSTER DOPPLER_PROJECT DOPPLER_CONFIG DEPLOY_CONFIG DEPLOY_ENV, Variable is not set)
	@bash $(SHARED_SCRIPT) info "Performing K8S deployment to $(DEPLOY_ENV)..."
	aws eks update-kubeconfig --name $(K8S_CLUSTER) --region $(AWS_REGION) || (echo "Failed to update kubeconfig" && exit 1)
	@bash $(SHARED_SCRIPT) info "Previous image: $(shell bash $(KSP_SCRIPT) image $(KSP_SERVICE))"
ifeq ($(DEPLOY_ENV),STG)
	@TARGET="deploy/stg" bash $(SHARED_SCRIPT) notify $(STG_DEPLOYMENT_MSG)
else ifeq ($(DEPLOY_ENV),PRD)
	@TARGET="deploy/prd" bash $(SHARED_SCRIPT) notify $(PRD_DEPLOYMENT_MSG)
endif
	doppler secrets substitute -p $(DOPPLER_PROJECT) -c $(DOPPLER_CONFIG) $(DEPLOY_CONFIG) | \
	sed "s/__VERSION__/$(VERSION)/g" | \
	sed "s/__SERVICE__/$(SERVICE)/g" | \
	sed "s/__AWS_ECR_URL__/$(AWS_ECR_URL)/g" | \
	kubectl apply -f -
