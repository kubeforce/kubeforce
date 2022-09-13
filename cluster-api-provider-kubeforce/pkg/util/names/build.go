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

package names

import "fmt"

const (
	maxNameLength = 63
)

// BuildName builds a valid name from the base name, adding a suffix to the
// the base. If base is valid, the returned name must also be valid.
func BuildName(base, suffix string) string {
	maxGeneratedNameLength := maxNameLength - len(suffix)
	if len(base) > maxGeneratedNameLength {
		base = base[:maxGeneratedNameLength]
	}
	return fmt.Sprintf("%s%s", base, suffix)
}
