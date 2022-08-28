MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MKFILE_DIR := $(realpath $(patsubst %/,%,$(dir $(MKFILE_PATH))))
BUILD_DIR ?= $(MKFILE_DIR)/_build
VERSION := $(shell $(MKFILE_DIR)/hack/version.sh version)

.PHONY: help
help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[0-9A-Za-z_-]+:.*?##/ { printf "  \033[36m%-45s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: all
all: tarball ## Build the binaries and docker images

.PHONY: test
test: agent-test controller-test ## Run unit tests with coverage analysis

.PHONY: agent-test
agent-test: ## Run the agent unit tests
	$(MAKE) -C agent test

.PHONY: agent
agent: ## Build the agent binaries
	$(MAKE) -C agent agent-build-linux-all

.PHONY: controller-test
controller-test: ## Run the controller unit tests
	$(MAKE) -C cluster-api-provider-kubeforce test

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

.PHONY: lint
lint: ## Lint packages on host
	@golangci-lint run \
		--config .golangci.yml \
		--timeout 10m \
		./agent/... \
		./cluster-api-provider-kubeforce/...


