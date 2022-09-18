/*
Copyright 2022 The Kubeforce Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"k3f.io/kubeforce/hack/tools/releasenotes/pkg"
)

var (
	fromTag = flag.String("from", "", "The tag or commit to start from.")
	version = flag.String("version", "", "The version of the release notes.")
	output  = flag.String("output", "", "The output path to the release notes.")

	tagRelease = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Println(err)
		if err, ok := err.(stackTracer); ok {
			for _, f := range err.StackTrace() {
				fmt.Printf("%+v\n", f)
			}
		}
		os.Exit(1)
	}
}

func lastReleaseTag() (string, error) {
	if fromTag != nil && *fromTag != "" {
		return *fromTag, nil
	}
	cmd := exec.Command("git", "tag", "--merged=HEAD", "--sort=-creatordate")
	out, err := cmd.Output()
	if err != nil {
		return firstCommit()
	}
	tags := strings.Split(strings.ReplaceAll(string(bytes.TrimSpace(out)), "\r\n", "\n"), "\n")
	for _, tag := range tags {
		if tag != *version && tagRelease.MatchString(tag) {
			return tag, nil
		}
	}
	return firstCommit()
}

func firstCommit() (string, error) {
	cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "unable to get first commit")
	}
	return string(bytes.TrimSpace(out)), nil
}

func validate() error {
	if *version == "" {
		return errors.Errorf("version is not specified")
	}
	if *output == "" {
		return errors.Errorf("output is not specified")
	}
	return nil
}

func run() error {
	if err := validate(); err != nil {
		return errors.WithStack(err)
	}
	lastTag := *fromTag
	if lastTag == "" {
		var err error
		lastTag, err = lastReleaseTag()
		if err != nil {
			return errors.WithStack(err)
		}
	}
	cfg := pkg.DefaultConfig()
	cfg.GitRange.From = lastTag
	cfg.Output = *output

	generator := pkg.NewGenerator(*cfg)
	if err := generator.Run(); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
