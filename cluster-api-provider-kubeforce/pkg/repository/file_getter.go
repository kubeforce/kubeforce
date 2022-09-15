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

package repository

import (
	"context"
)

// File describes the file that can be obtained from the FileGetter.
type File struct {
	// Path is the full path to the cached file in the file system.
	Path string
}

// FileGetter is an interface to get files from different sources.
type FileGetter interface {
	// GetFile returns file by relativePath.
	GetFile(ctx context.Context, relativePath string) (*File, error)
	// RemoveCache removes all files from the cache.
	RemoveCache() error
}
