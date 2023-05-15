#!/usr/bin/env bash

# Copyright 2021 The Kubeforce Authors.
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

set -o errexit
set -o nounset
set -o pipefail

ROOT_MODULE_NAME=k3f.io/kubeforce
MODULE_NAME=${ROOT_MODULE_NAME}/agent
CODEGEN_TAG_VERSION=v0.26.4
CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$( cd "${CURRENT_DIR}/../../" && pwd )"
CURRENT_DIR=$(dirname "${BASH_SOURCE[0]}")
GOPATH_SRC="${GOPATH}/src"
go mod download k8s.io/code-generator@${CODEGEN_TAG_VERSION}
CODEGEN_PKG=${GOPATH}/pkg/mod/k8s.io/code-generator@${CODEGEN_TAG_VERSION}

go install k8s.io/code-generator/cmd/{defaulter-gen,conversion-gen,client-gen,lister-gen,informer-gen,deepcopy-gen,openapi-gen}@${CODEGEN_TAG_VERSION}
cd "${GOPATH_SRC}/${ROOT_MODULE_NAME}"

bash "${CODEGEN_PKG}/generate-groups.sh" all \
  ${MODULE_NAME}/pkg/generated ${MODULE_NAME}/pkg/apis \
  "agent:v1alpha1" \
  --output-base "${GOPATH_SRC}" \
  --go-header-file "${ROOT_DIR}/hack/boilerplate/boilerplate.generatego.txt"

bash "${CURRENT_DIR}/generate-internal-groups.sh" "deepcopy,defaulter,conversion,openapi" \
  ${MODULE_NAME}/pkg/generated ${MODULE_NAME}/pkg/apis ${MODULE_NAME}/pkg/apis \
  "agent:v1alpha1" \
  --output-base "${GOPATH_SRC}" \
  --go-header-file "${ROOT_DIR}/hack/boilerplate/boilerplate.generatego.txt"

bash "${CURRENT_DIR}/generate-internal-groups.sh" "deepcopy,defaulter,conversion" \
  ${MODULE_NAME}/pkg/config/generated ${MODULE_NAME}/pkg/ ${MODULE_NAME}/pkg/ \
  "config:v1alpha1" \
  --output-base "${GOPATH_SRC}" \
  --go-header-file "${ROOT_DIR}/hack/boilerplate/boilerplate.generatego.txt"
