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

version::get_version_vars() {
    # shellcheck disable=SC1083
    GIT_COMMIT="$(git rev-parse HEAD^{commit})"
    if git_status=$(git status --porcelain 2>/dev/null) && [[ -z ${git_status} ]]; then
        GIT_TREE_STATE="clean"
    else
        GIT_TREE_STATE="dirty"
    fi

    last_tag=$(git describe --abbrev=0 --tags)
    commit_hash=$(git rev-parse HEAD | cut -c-9)
    git_version=$(git describe --tags)
    commits=$(git rev-list  "${last_tag}..HEAD" --count)

    if [[ "${last_tag}" == "${git_version}" ]] ; then  # the current commit is a release
        GIT_VERSION="${last_tag}"
    elif [[ "${last_tag}" != *-* ]] ; then  # the last tag is not pre-release version
        GIT_VERSION="$(increment_patch "${last_tag}")-alpha.0.${commits}.${commit_hash}"
    else  # the current ref is a descendent of a pre-release version (e.g. already an rc, alpha, or beta)
        GIT_VERSION="${last_tag}.${commits}.${commit_hash}"
    fi
    if [[ "${GIT_TREE_STATE-}" == "dirty" ]]; then
        # git describe --dirty only considers changes to existing files, but
        # that is problematic since new untracked .go files affect the build,
        # so use our idea of "dirty" from git status instead.
        GIT_VERSION+="-dirty"
    fi

    # Try to match the "git describe" output to a regex to try to extract
    # the "major" and "minor" versions and whether this is the exact tagged
    # version or whether the tree is between two tagged versions.
    if [[ "${GIT_VERSION}" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)?([-].*)?([+].*)?$ ]]; then
        GIT_MAJOR=${BASH_REMATCH[1]}
        GIT_MINOR=${BASH_REMATCH[2]}
    fi

    # If GIT_VERSION is not a valid Semantic Version, then refuse to build.
    if ! [[ "${GIT_VERSION}" =~ ^v([0-9]+)\.([0-9]+)(\.[0-9]+)?(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
        echo "GIT_VERSION should be a valid Semantic Version. Current value: ${GIT_VERSION}"
        echo "Please see more details here: https://semver.org"
        exit 1
    fi

    GIT_RELEASE_TAG=$(git describe --abbrev=0 --tags)
    GIT_RELEASE_COMMIT=$(git rev-list -n 1  "${GIT_RELEASE_TAG}")
}

increment_patch() {
    local major minor patch version
    version=$1
    if [[ "$version" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)?([-].*)?([+].*)?$ ]]; then
        major=${BASH_REMATCH[1]}
        minor=${BASH_REMATCH[2]}
        patch=${BASH_REMATCH[3]}
    else
      echo "version should be a valid Semantic Version. Current value: ${version}"
      echo "Please see more details here: https://semver.org"
      exit 1
    fi
    patch=$((patch + 1))
    echo "v${major}.${minor}.${patch}"
}

version::git_version() {
    version::get_version_vars
    echo ${GIT_VERSION}
}

# stolen from k8s.io/hack/lib/version.sh and modified
# Prints the value that needs to be passed to the -ldflags parameter of go build
version::ldflags() {
    version::get_version_vars

    local -a ldflags
    function add_ldflag() {
        local key=${1}
        local val=${2}
        ldflags+=(
            "-X 'k8s.io/component-base/version.${key}=${val}'"
        )
    }

    add_ldflag "buildDate" "$(date ${SOURCE_DATE_EPOCH:+"--date=@${SOURCE_DATE_EPOCH}"} -u +'%Y-%m-%dT%H:%M:%SZ')"
    add_ldflag "gitCommit" "${GIT_COMMIT}"
    add_ldflag "gitTreeState" "${GIT_TREE_STATE}"
    add_ldflag "gitMajor" "${GIT_MAJOR}"
    add_ldflag "gitMinor" "${GIT_MINOR}"
    add_ldflag "gitVersion" "${GIT_VERSION}"
    add_ldflag "gitReleaseCommit" "${GIT_RELEASE_COMMIT}"

    # The -ldflags parameter takes a single string, so join the output.
    echo "${ldflags[*]-}"
}

main() {
  case $1 in
        "ldflags")
          version::ldflags
                ;;
        "version")
          version::git_version
                ;;
  esac
}

main $1
