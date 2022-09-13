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

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

ARCH ?= amd64
ALL_ARCH = amd64 arm arm64

# Directories.
MKFILE_PATH := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
ROOT_DIR := $(abspath $(MKFILE_PATH)/..)
BUILD_DIR ?= $(ROOT_DIR)/_build
TOOLS_BIN_DIR := $(abspath $(BUILD_DIR)/tools/bin)
VERSION := $(shell $(ROOT_DIR)/hack/version.sh version)
BIN_DIR ?= $(BUILD_DIR)/$(VERSION)/bin
AGENT_DIR := $(BIN_DIR)/linux-$(ARCH)
TILTBUILD_DIR := $(MKFILE_PATH)/.tiltbuild

# Set build time variables including version details
LDFLAGS := $(shell $(ROOT_DIR)/hack/version.sh ldflags)

# Define Docker related variables. Releases should modify and double check these vars.
REGISTRY ?= ghcr.io/kubeforce
IMAGE_NAME ?= cluster-api-kubeforce-controller
# Image URL to use all building/pushing image targets
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
TAG ?= $(shell $(ROOT_DIR)/hack/version.sh version)

#
# Tools.
#
KUSTOMIZE_VER := v4.5.2
KUSTOMIZE_BIN := kustomize
KUSTOMIZE := $(abspath $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)-$(KUSTOMIZE_VER))
KUSTOMIZE_PKG := sigs.k8s.io/kustomize/kustomize/v4

SETUP_ENVTEST_VER := v0.0.0-20211110210527-619e6b92dab9
SETUP_ENVTEST_BIN := setup-envtest
SETUP_ENVTEST := $(abspath $(TOOLS_BIN_DIR)/$(SETUP_ENVTEST_BIN)-$(SETUP_ENVTEST_VER))
SETUP_ENVTEST_PKG := sigs.k8s.io/controller-runtime/tools/setup-envtest

CONTROLLER_GEN_VER := v0.8.0
CONTROLLER_GEN_BIN := controller-gen
CONTROLLER_GEN := $(abspath $(TOOLS_BIN_DIR)/$(CONTROLLER_GEN_BIN)-$(CONTROLLER_GEN_VER))
CONTROLLER_GEN_PKG := sigs.k8s.io/controller-tools/cmd/controller-gen

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: $(CONTROLLER_GEN) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) \
		paths="./..." \
		crd:crdVersions=v1 \
		rbac:roleName=manager-role \
		output:crd:dir=./config/crd/bases \
		output:rbac:dir=./config/rbac \
		output:webhook:dir=./config/webhook \
		webhook

generate: $(CONTROLLER_GEN) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="$(ROOT_DIR)/hack/boilerplate/boilerplate.generatego.txt" paths="./..."

.PHONY: tilt-agents
tilt-agents: ## Build agent binaries for tilt
	mkdir -p $(TILTBUILD_DIR)/bin/kubeforce
	BUILD_DIR=$(TILTBUILD_DIR)/bin/kubeforce $(MAKE) -C $(ROOT_DIR)/agent agent-build-all

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

## --------------------------------------
## Testing
## --------------------------------------

##@ test:

COVERAGE_DIR := $(BUILD_DIR)/coverage

ifeq ($(shell go env GOOS),darwin) # Use the darwin/amd64 binary until an arm64 version is available
	KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env -p path --arch amd64 $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))
else
	KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env -p path $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))
endif

.PHONY: test
test: $(SETUP_ENVTEST) ## Run unit and integration tests
	KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" go test ./... $(TEST_ARGS)

.PHONY: test-verbose
test-verbose: ## Run unit and integration tests with verbose flag
	$(MAKE) test TEST_ARGS="$(TEST_ARGS) -v"

.PHONY: test-cover
test-cover: ## Run unit and integration tests and generate a coverage report
	mkdir -p $(COVERAGE_DIR)
	$(MAKE) test TEST_ARGS="$(TEST_ARGS) -coverprofile=$(COVERAGE_DIR)/coverage.out"
	go tool cover -func=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.txt
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html

## --------------------------------------
## Build
## --------------------------------------

##@ Build

build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: docker-build-all
docker-build-all: $(addprefix docker-build-,$(ALL_ARCH)) ## Build docker images for all architectures

docker-build-%:
	$(MAKE) ARCH=$* docker-build

.PHONY: docker-build
docker-build: ## Build the docker image for the manager
	DOCKER_BUILDKIT=1 BUILDKIT_PROGRESS=plain docker build \
	--build-arg goproxy=$(GOPROXY) \
	--build-arg ARCH=$(ARCH) \
	--build-arg LDFLAGS="$(LDFLAGS)" \
	-f $(MKFILE_PATH)/Dockerfile \
	$(ROOT_DIR) -t $(CONTROLLER_IMG)-$(ARCH):$(TAG)

.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(CONTROLLER_IMG)-$(ARCH):$(TAG)

.PHONY: docker-push-all ## Push all the architecture docker images
docker-push-all: $(addprefix docker-push-,$(ALL_ARCH))
	$(MAKE) docker-push-manifest

docker-push-%: ## Docker push
	$(MAKE) ARCH=$* docker-push

.PHONY: docker-push-manifest
docker-push-manifest: ## Push the manifest image
	docker manifest create --amend $(CONTROLLER_IMG):$(TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(CONTROLLER_IMG)\-&:$(TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${CONTROLLER_IMG}:${TAG} ${CONTROLLER_IMG}-$${arch}:${TAG}; done
	docker manifest push --purge ${CONTROLLER_IMG}:${TAG}

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for manager resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' $(TARGET_RESOURCE)

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for manager resources)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' $(TARGET_RESOURCE)

.PHONY: clean-release-git
clean-release-git: ## Restores the git files usually modified during a release
	git restore ./*manager_image_patch.yaml ./*manager_pull_policy.yaml

## --------------------------------------
## Release
## --------------------------------------

##@ release:

# the previous release tag, e.g., v0.3.9, excluding pre-release tags
## set by Prow, ref name of the base branch, e.g., main
RELEASE_DIR ?= $(BUILD_DIR)/release/$(VERSION)

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

.PHONY: release
release: ## Build and push container images using the latest git tag for the commit
	@if [ -z "${VERSION}" ]; then echo "VERSION is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	# Build binaries first.
	# Set the manifest image to the production bucket.
	$(MAKE) manifest-modification
	## Build the manifests
	RELEASE_DIR=$(RELEASE_DIR) $(MAKE) release-manifests
	## Clean the git artifacts modified in the release process
	$(MAKE) clean-release-git

.PHONY: manifest-modification
manifest-modification: # Set the manifest images to the staging/production bucket.
	$(MAKE) set-manifest-image \
		MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(VERSION) \
		TARGET_RESOURCE="./config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"

.PHONY: release-manifests
release-manifests: $(RELEASE_DIR) $(KUSTOMIZE) ## Build the manifests to publish with a release
	# Build infrastructure-components.
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml

## --------------------------------------
## Deployment
## --------------------------------------

##@ Deployment

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	$(MAKE) set-manifest-image \
		MANIFEST_IMG=$(CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) \
		TARGET_RESOURCE="./config/default/manager_image_patch.yaml"
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

.PHONY: clean
clean: ## Remove build artefacts
	rm -rf $(BUILD_DIR)

## --------------------------------------
## Hack / Tools
## --------------------------------------

##@ hack/tools:

.PHONY: $(CONTROLLER_GEN_BIN)
$(CONTROLLER_GEN_BIN): $(CONTROLLER_GEN) ## Build a local copy of controller-gen.

.PHONY: $(KUSTOMIZE_BIN)
$(KUSTOMIZE_BIN): $(KUSTOMIZE) ## Build a local copy of kustomize.

$(KUSTOMIZE): # Build kustomize from tools folder.
	CGO_ENABLED=0 GOBIN=$(TOOLS_BIN_DIR) go install $(KUSTOMIZE_PKG)@$(KUSTOMIZE_VER)
	mv $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN) $(KUSTOMIZE)

$(CONTROLLER_GEN): # Build controller-gen from tools folder.
	CGO_ENABLED=0 GOBIN=$(TOOLS_BIN_DIR) go install $(CONTROLLER_GEN_PKG)@$(CONTROLLER_GEN_VER)
	mv $(TOOLS_BIN_DIR)/$(CONTROLLER_GEN_BIN) $(CONTROLLER_GEN)

.PHONY: $(SETUP_ENVTEST_BIN)
$(SETUP_ENVTEST_BIN): $(SETUP_ENVTEST) ## Build a local copy of setup-envtest.

$(SETUP_ENVTEST): # Build setup-envtest from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) go install $(SETUP_ENVTEST_PKG)@$(SETUP_ENVTEST_VER)
	mv $(TOOLS_BIN_DIR)/$(SETUP_ENVTEST_BIN) $(SETUP_ENVTEST)
