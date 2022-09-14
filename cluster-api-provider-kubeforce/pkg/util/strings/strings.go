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

package strings

func Find(fn FilterFn, vars ...string) string {
	for _, s := range vars {
		if fn(s) {
			return s
		}
	}
	return ""
}

type FilterFn func(s string) bool

func IsNotEmpty(s string) bool {
	return s != ""
}

func Filter(fn FilterFn, vars ...string) []string {
	result := make([]string, 0, len(vars))
	for _, s := range vars {
		if fn(s) {
			result = append(result, s)
		}
	}
	return result
}
