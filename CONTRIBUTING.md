# Contributing Guidelines

- [Finding Things That Need Help](#finding-things-that-need-help)
- [Contributing a Patch](#contributing-a-patch)
- [Branches](#branches)

Read the following guide if you're interested in contributing to Kubeforce provider.

## Finding Things That Need Help

If you're new to the project and want to help, but don't know where to start, we have a semi-curated list of issues that
should not need deep knowledge of the system. [Have a look and see if anything sounds
interesting](https://github.com/kubeforce/kubeforce/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22).
Before starting to work on the issue, make sure that it doesn't have a [lifecycle/active](https://github.com/kubeforce/kubeforce/labels/lifecycle%2Factive) label. If the issue has been assigned, reach out to the assignee.
Alternatively, read some of the docs on other controllers and try to write your own, file and fix any/all issues that
come up, including gaps in documentation!

Help and contributions are very welcome in the form of code contributions but also in helping to moderate office hours, triaging issues, fixing/investigating flaky tests, cutting releases, helping new contributors with their questions, reviewing proposals, etc.

## Contributing a Patch

1. If working on an issue, signal other contributors that you are actively working on it.
1. Fork the desired repo, develop and test your code changes.
1. Submit a pull request.
    1. All code PR must be labeled with one of
        - ✨️ (`:sparkles:` New Features)
        - 🔒 (`:lock:` Fix security issues)
        - 🐛 (`:bug:` Bug Fixes)
        - 🎨 (`:art:` Improve structure / format of the code)
        - ♻ (`:recycle:` Refactor code)
        - 📝 (`:memo:` Documentation)
        - ⚠️ (`:warning:` Breaking Changes)
        - 🔧 (`:wrench:` Add or update configuration files)
        - 🌱 (`:seedling:` Others)
1. If your PR has multiple commits, you must before merging your PR.

All changes must be code reviewed. Coding conventions and standards are explained in the official [developer
docs](https://git.k8s.io/community/contributors/devel). Expect reviewers to request that you
avoid common [go style mistakes](https://github.com/golang/go/wiki/CodeReviewComments) in your PRs.

## Branches

Kubeforce has two types of branches: the *main* branch and
*release-X* branches.

The *main* branch is where development happens. All the latest and
greatest code, including breaking changes, happens on main.

The *release-X* branches contain stable, backwards compatible code. On every
major or minor release, a new branch is created. It is from these
branches that minor and patch releases are tagged. In some cases, it may
be necessary to open PRs for bugfixes directly against stable branches, but
this should generally not be the case.