# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL:=/usr/bin/env bash

.DEFAULT_GOAL:=help

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# This option is for running docker manifest command
export DOCKER_CLI_EXPERIMENTAL := enabled

ARCH ?= amd64
GOOS ?= linux
ALL_LINUX_ARCH = amd64 arm arm64
ALL_DARWIN_ARCH = amd64 arm64

# Directories.
MKFILE_PATH := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
ROOT_DIR := $(abspath $(MKFILE_PATH)/..)
BUILD_DIR ?= $(ROOT_DIR)/_build
TOOLS_BIN_DIR := $(abspath $(BUILD_DIR)/tools/bin)
VERSION := $(shell $(ROOT_DIR)/hack/version.sh version)
BIN_DIR ?= $(BUILD_DIR)/$(VERSION)/bin

REPOSITORY_IMG ?= localhost:5000/agent-repository
TAG ?= $(VERSION)

# Set build time variables including version details
LDFLAGS := $(shell $(ROOT_DIR)/hack/version.sh ldflags)

#
# Tools.
#
KUSTOMIZE_VER := v4.5.2
KUSTOMIZE_BIN := kustomize
KUSTOMIZE := $(abspath $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)-$(KUSTOMIZE_VER))
KUSTOMIZE_PKG := sigs.k8s.io/kustomize/kustomize/v4

all: test agent-build-all

.PHONY: help
help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[0-9A-Za-z_-]+:.*?##/ { printf "  \033[36m%-45s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------

.PHONY: test
test: ## Run tests
	go test -v ./...

.PHONY: test-integration
test-integration: ## Run integration tests
	source ./scripts/fetch_ext_bins.sh; fetch_tools; setup_envs; go test -v -tags=integration ./test/integration/...

## --------------------------------------
## Binaries
## --------------------------------------

.PHONY: agent-build-all ## Build all the architecture of agent
agent-build-all: agent-build-linux-all agent-build-darwin-all

.PHONY: agent-build-linux-all ## Build all the architecture of agent for linux
agent-build-linux-all: $(addprefix agent-build-linux-,$(ALL_LINUX_ARCH))

.PHONY: agent-build-darwin-all ## Build all the architecture of agent for darwin
agent-build-darwin-all: $(addprefix agent-build-darwin-,$(ALL_DARWIN_ARCH))

agent-build-linux-%:
	GOOS=linux ARCH=$* $(MAKE) agent-build

agent-build-darwin-%:
	GOOS=darwin ARCH=$* $(MAKE) agent-build

.PHONY: agent-build
agent-build:
	mkdir -p $(BIN_DIR)
	GOOS=$(GOOS) GOARCH=$(ARCH) CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/agent-$(GOOS)-$(ARCH)

## --------------------------------------
## Artifact repository for development
## --------------------------------------

.PHONY: repository-build
repository-build: agent-build-linux-all ## Build the repository for development
	DOCKER_BUILDKIT=1 BUILDKIT_PROGRESS=plain docker build \
	--build-arg VERSION="$(VERSION)" \
	-f $(MKFILE_PATH)/hack/repository/Dockerfile \
	$(ROOT_DIR) -t $(REPOSITORY_IMG):$(TAG)

.PHONY: repository-deploy
repository-deploy: $(KUSTOMIZE) repository-build repository-push ## Deploy artifact repository to the K8s cluster specified in ~/.kube/config.
	cd $(MKFILE_PATH)/hack/repository/config/repository && $(KUSTOMIZE) edit set image agent-repository=$(REPOSITORY_IMG):$(TAG)
	$(KUSTOMIZE) build hack/repository/config/default | kubectl apply -f -
	git restore $(MKFILE_PATH)/hack/repository/config/repository/kustomization.yaml

.PHONY: repository-undeploy
repository-undeploy: ## Undeploy artifact repository from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build hack/repository/config/default | kubectl delete -f -

.PHONY: repository-push
repository-push: ## Push the agent-repository docker image
	docker push $(REPOSITORY_IMG):$(TAG)

## --------------------------------------
## Generate kube resources
## --------------------------------------

.PHONY: gen-client
gen-client: ## Generate client code
	./hack/docker-update-codegen.sh
	git add pkg/generated

## --------------------------------------
## Linting
## --------------------------------------

.PHONY: lint lint-full
lint: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v
	cd $(E2E_FRAMEWORK_DIR); $(GOLANGCI_LINT) run -v

lint-full: $(GOLANGCI_LINT) ## Run slower linters to detect possible issues
	$(GOLANGCI_LINT) run -v --fast=false
	cd $(E2E_FRAMEWORK_DIR); $(GOLANGCI_LINT) run -v --fast=false

## --------------------------------------
## Hack / Tools
## --------------------------------------

##@ hack/tools:

.PHONY: $(KUSTOMIZE_BIN)
$(KUSTOMIZE_BIN): $(KUSTOMIZE) ## Build a local copy of kustomize.

$(KUSTOMIZE): # Build kustomize from tools folder.
	CGO_ENABLED=0 GOBIN=$(TOOLS_BIN_DIR) go install $(KUSTOMIZE_PKG)@$(KUSTOMIZE_VER)
	mv $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN) $(KUSTOMIZE)

## --------------------------------------
## Cleanup
## --------------------------------------

.PHONY: clean
clean: ## Remove all generated files
	$(MAKE) clean-bin

.PHONY: clean-bin
clean-bin: ## Remove all generated binaries
	rm -rf $(BUILD_DIR)

