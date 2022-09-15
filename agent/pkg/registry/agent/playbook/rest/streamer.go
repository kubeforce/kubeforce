/*
Copyright 2021 The Kubeforce Authors.

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

package rest

import (
	"context"
	"fmt"
	"io"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"
)

// FileStreamer is a resource that streams the contents of a particular file.
type FileStreamer struct {
	Path        string
	ContentType string
	Flush       bool
}

// a FileStreamer must implement a rest.ResourceStreamer.
var _ rest.ResourceStreamer = &FileStreamer{}

// GetObjectKind returns the kind of object reference.
func (s *FileStreamer) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns the deep copy of the object.
func (s *FileStreamer) DeepCopyObject() runtime.Object {
	panic("rest.FileStreamer does not implement DeepCopyObject")
}

// InputStream returns a stream with the contents of the file. If no location is provided,
// a null stream is returned.
func (s *FileStreamer) InputStream(ctx context.Context, apiVersion, acceptHeader string) (stream io.ReadCloser, flush bool, contentType string, err error) {
	if s.Path == "" {
		// If no location was provided, return a null stream
		return nil, false, s.ContentType, nil
	}

	f, err := os.Open(s.Path)
	if err != nil {
		return nil, false, s.ContentType, fmt.Errorf("unable to read file: %s err: %v", s.Path, err)
	}

	return f, s.Flush, s.ContentType, nil
}
