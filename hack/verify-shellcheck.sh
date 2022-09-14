#!/usr/bin/env bash

# Copyright 2022 The Kubeforce Authors.
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

# This script lints each shell script by `shellcheck`.
# Usage: `hack/verify-shellcheck.sh`.

set -o errexit
set -o nounset
set -o pipefail

VERSION="v0.8.0"

OS="unknown"
if [[ "${OSTYPE}" == "linux"* ]]; then
  OS="linux"
elif [[ "${OSTYPE}" == "darwin"* ]]; then
  OS="darwin"
fi

ARCH=$(uname -m)
# get_root_path returns the root path of the project source tree
get_root_path() {
    git rev-parse --show-toplevel
}

ROOT_PATH=$(get_root_path)
SHELLCHECK="${ROOT_PATH}/_build/tools/bin/shellcheck-${VERSION}"

# create a temporary directory
TMP_DIR=$(mktemp -d)
OUT="${TMP_DIR}/out.log"

# cleanup on exit
cleanup() {
  ret=0
  if [[ -s "${OUT}" ]]; then
    echo "Found errors:"
    cat "${OUT}"
    ret=1
  fi
  rm -rf "${TMP_DIR}"
  if [ $ret -eq 0 ]; then
    echo "Congratulations! All shell files are passing lint :-)"
  fi
  exit ${ret}
}
trap cleanup EXIT


if [ ! -f "$SHELLCHECK" ]; then
  # install shellcheck
  cd "${TMP_DIR}" || exit
  DOWNLOAD_FILE="shellcheck-${VERSION}.${OS}.${ARCH}.tar.xz"
  curl -L "https://github.com/koalaman/shellcheck/releases/download/${VERSION}/${DOWNLOAD_FILE}" -o "${TMP_DIR}/shellcheck.tar.xz"
  tar xf "${TMP_DIR}/shellcheck.tar.xz"
  cd "${ROOT_PATH}"
  mkdir -p "${ROOT_PATH}/_build/tools/bin"
  mv "${TMP_DIR}/shellcheck-${VERSION}/shellcheck" "$SHELLCHECK"
fi

echo "Running shellcheck..."
cd "${ROOT_PATH}" || exit

FILES=$(find . -name "*.sh" -not -path "./tmp/*" -not -path "./_build/*")
while read -r file; do
    "$SHELLCHECK" -x "$file" >> "${OUT}" 2>&1
done <<< "$FILES"
