# Copyright 2018 The Kubeforce Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# If you update this file, please follow
# https://www.thapaliya.com/en/writings/well-documented-makefiles/

MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MKFILE_DIR := $(realpath $(patsubst %/,%,$(dir $(MKFILE_PATH))))
BUILD_DIR ?= $(MKFILE_DIR)/_build
VERSION := $(shell $(MKFILE_DIR)/hack/version.sh version)
INTERNAL_TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(abspath $(BUILD_DIR)/tools/bin)

#
# Tools.
#
GOLINTCI_LINT_VER := v1.50.1
GOLINTCI_LINT_BIN := golangci-lint
GOLINTCI_LINT := $(abspath $(TOOLS_BIN_DIR)/$(GOLINTCI_LINT_BIN)-$(GOLINTCI_LINT_VER))
GOLINTCI_LINT_PKG := github.com/golangci/golangci-lint/cmd/golangci-lint

HADOLINT_VER := v2.12.0
HADOLINT_FAILURE_THRESHOLD = warning

.PHONY: help
help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[0-9A-Za-z_-]+:.*?##/ { printf "  \033[36m%-45s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------

##@ test:

.PHONY: test
test: agent-test controller-test ## Run unit tests with coverage analysis

.PHONY: agent-test
agent-test: ## Run the agent unit tests
	$(MAKE) -C agent test

.PHONY: controller-test
controller-test: ## Run the controller unit tests
	$(MAKE) -C cluster-api-provider-kubeforce test

## --------------------------------------
## Building
## --------------------------------------

##@ build:

.PHONY: agent
agent: ## Build the agent binaries
	$(MAKE) -C agent agent-build-linux-all

.PHONY: controller-build
controller-build: ## Build the controller docker images
	$(MAKE) -C cluster-api-provider-kubeforce docker-build

.PHONY: controller-build-all
controller-build-all: ## Build the controller docker images for all architectures
	$(MAKE) -C cluster-api-provider-kubeforce docker-build-all

.PHONY: controller-push-all
controller-push-all: ## Push the controller docker images for all architectures
	$(MAKE) -C cluster-api-provider-kubeforce docker-push-all

## --------------------------------------
## Release
## --------------------------------------

##@ release:

ifneq (,$(findstring -,$(VERSION)))
    PRE_RELEASE=true
endif
# the previous release tag, e.g., v0.3.9, excluding pre-release tags
## set by Prow, ref name of the base branch, e.g., main
RELEASE_DIR := $(BUILD_DIR)/release/$(VERSION)
RELEASE_NOTES_DIR := $(BUILD_DIR)/releasenotes/$(VERSION)

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

$(RELEASE_NOTES_DIR):
	mkdir -p $(RELEASE_NOTES_DIR)/

.PHONY: release
release: $(RELEASE_DIR) $(RELEASE_NOTES_DIR) ## Build release artifacts using the latest git tag for the commit
	@if [ -z "${VERSION}" ]; then echo "VERSION is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	$(MAKE) -C cluster-api-provider-kubeforce release
	BIN_DIR=$(RELEASE_DIR) $(MAKE) -C agent agent-build-linux-all

.PHONY: release-notes
release-notes: $(RELEASE_NOTES_DIR)
	@if [ -n "${PRE_RELEASE}" ]; then \
	echo ":rotating_light: This is a RELEASE CANDIDATE. Use it only for testing purposes. If you find any bugs, file an [issue](https://github.com/kubeforce/kubeforce/issues/new)." > $(RELEASE_NOTES_DIR)/RELEASE_NOTES.md; \
	else \
	cd ./hack/tools/releasenotes/ && go run main.go --version=$(VERSION) --output $(RELEASE_NOTES_DIR)/RELEASE_NOTES.md; \
	fi

$(BUILD_DIR):
	mkdir -p $@

.PHONY: clean
clean: ## Remove build artefacts
	rm -rf $(BUILD_DIR)

## --------------------------------------
## Generate / Manifests
## --------------------------------------

##@ generate:

.PHONY: generate
generate: ## Run all generation targets
	$(MAKE) -C cluster-api-provider-kubeforce generate manifests
	$(MAKE) -C agent gen-client

.PHONY: generate-modules
generate-modules: ## Run go mod tidy to ensure modules are up to date
	go mod tidy
	cd $(INTERNAL_TOOLS_DIR); go mod tidy

## --------------------------------------
## Lint / Verify
## --------------------------------------

##@ lint and verify:

.PHONY: lint
lint: $(GOLINTCI_LINT) ## Lint packages on host
	$(GOLINTCI_LINT) run -v --timeout 10m ./...
	cd $(INTERNAL_TOOLS_DIR); $(GOLINTCI_LINT) run -v --timeout 10m ./...


ALL_VERIFY_CHECKS = modules boilerplate shellcheck modules dockerfiles gen

.PHONY: verify
verify: $(addprefix verify-,$(ALL_VERIFY_CHECKS)) ## Run all verify-* targets

.PHONY: verify-boilerplate
verify-boilerplate: ## Verify boilerplate text exists in each file
	./hack/verify-boilerplate.sh

.PHONY: verify-modules
verify-modules: generate-modules  ## Verify go modules are up to date
	@if !(git diff --quiet HEAD -- go.sum go.mod $(INTERNAL_TOOLS_DIR)/go.mod $(INTERNAL_TOOLS_DIR)/go.sum); then \
		git --no-pager diff; \
		echo "go module files are out of date"; exit 1; \
	fi
	@if (find . -name 'go.mod' | xargs -n1 grep -q -i 'k8s.io/client-go.*+incompatible'); then \
		find . -name "go.mod" -exec grep -i 'k8s.io/client-go.*+incompatible' {} \; -print; \
		echo "go module contains an incompatible client-go version"; exit 1; \
	fi

.PHONY: verify-gen
verify-gen: generate  ## Verify go generated files are up to date
	@if !(git diff --quiet HEAD); then \
		git --no-pager diff; \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi

.PHONY: verify-shellcheck
verify-shellcheck: ## Verify shell files
	./hack/verify-shellcheck.sh

.PHONY: verify-dockerfiles
verify-dockerfiles: ## Verify dockerfiles
	./hack/verify-dockerfiles.sh $(HADOLINT_VER) $(HADOLINT_FAILURE_THRESHOLD)

## --------------------------------------
## Hack / Tools
## --------------------------------------

##@ hack/tools:

.PHONY: $(GOLINTCI_LINT_BIN)
$(GOLINTCI_LINT_BIN): $(GOLINTCI_LINT) ## Build a local copy of golintci-lint.

$(GOLINTCI_LINT): # Build golintci-lint from tools folder.
	CGO_ENABLED=0 GOBIN=$(TOOLS_BIN_DIR) go install $(GOLINTCI_LINT_PKG)@$(GOLINTCI_LINT_VER)
	mv $(TOOLS_BIN_DIR)/$(GOLINTCI_LINT_BIN) $(GOLINTCI_LINT)
