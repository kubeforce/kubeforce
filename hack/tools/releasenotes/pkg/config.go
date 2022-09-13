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

package pkg

type Config struct {
	Output   string
	GitRange GitRange
	Headers  []Header
}

type GitRange struct {
	From string
	To   string
}

type Header struct {
	Name     string
	Prefixes []string
}

func DefaultConfig() *Config {
	return &Config{
		GitRange: GitRange{
			To: "HEAD",
		},
		Headers: []Header{
			{
				Name:     ":sparkles: New Features",
				Prefixes: []string{":sparkles:", "✨"},
			},
			{
				Name:     ":lock: Fix security issues",
				Prefixes: []string{":lock:", "🔒"},
			},
			{
				Name:     ":bug: Bug Fixes",
				Prefixes: []string{":bug:", "🐛"},
			},
			{
				Name:     ":art: Improve structure / format of the code",
				Prefixes: []string{":art:", "🎨"},
			},
			{
				Name:     ":recycle: Refactor code",
				Prefixes: []string{":recycle:", "♻️"},
			},
			{
				Name:     ":memo: Documentation",
				Prefixes: []string{":memo:", "📝"},
			},
			{
				Name:     ":warning: Breaking Changes",
				Prefixes: []string{":warning:", "⚠️"},
			},
			{
				Name:     ":wrench: Add or update configuration files",
				Prefixes: []string{":wrench:", "🔧"},
			},
			{
				Name:     ":seedling: Others",
				Prefixes: []string{":seedling:", "🌱"},
			},
		},
	}
}
