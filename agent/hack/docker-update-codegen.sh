#!/usr/bin/env bash

# Copyright 2021
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
set -x
set -o errexit
set -o nounset
set -o pipefail

ROOT_MODULE_NAME=k3f.io/kubeforce
CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="${CURRENT_DIR}/../../"

docker run -ti --rm \
-v "${ROOT_DIR}:/go/src/${ROOT_MODULE_NAME}" \
golang:1.16 \
/go/src/${ROOT_MODULE_NAME}/agent/hack/update-codegen.sh
