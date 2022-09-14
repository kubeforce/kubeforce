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

package ansible

import "context"

var _ PackageManager = &aptPkgManager{}

type aptPkgManager struct {
}

func (a aptPkgManager) Update(ctx context.Context) error {
	return runCmd(ctx, "sudo", "apt-get", "update")
}

func (a aptPkgManager) Install(ctx context.Context, packages ...string) error {
	args := append([]string{"sudo", "apt-get", "install", "-y"}, packages...)
	return runCmd(ctx, args...)
}
