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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
)

// NewHTTPFileGetter creates a FileGetter for HTTPRepository
func NewHTTPFileGetter(s *Storage, r infrav1.HTTPRepository) FileGetter {
	return &HTTPFileGetter{
		repository: r,
		storage:    s,
	}
}

type HTTPFileGetter struct {
	repository infrav1.HTTPRepository
	storage    *Storage
}

func convertURLToFilesystemPath(url string) string {
	result := strings.ReplaceAll(url, "://", "/")
	result = strings.ReplaceAll(result, ":", "_")
	result = strings.ReplaceAll(result, "&", "_")
	result = strings.ReplaceAll(result, "|", "_")
	result = strings.ReplaceAll(result, ">", "_")
	result = strings.ReplaceAll(result, "<", "_")
	return result
}

func (g *HTTPFileGetter) GetFile(ctx context.Context, relativePath string) (*File, error) {
	parsedURL, err := url.Parse(g.repository.Spec.URL)
	if err != nil {
		return nil, err
	}
	parsedURL.Path = path.Join(parsedURL.Path, relativePath)

	fileURL := parsedURL.String()
	relativeFSPath := path.Join(g.repository.Namespace, g.repository.Name, convertURLToFilesystemPath(fileURL))
	fullFilePath, err := g.storage.getFile(ctx, relativeFSPath, g.download(fileURL))
	if err != nil {
		return nil, err
	}
	return &File{
		Path: fullFilePath,
	}, nil
}

func (g *HTTPFileGetter) RemoveCache() error {
	relativeFSPath := path.Join(g.repository.Namespace, g.repository.Name)
	return g.storage.remove(relativeFSPath)
}

func (g *HTTPFileGetter) download(url string) downloader {
	return func(ctx context.Context, w io.Writer) error {
		if g.repository.Spec.Timeout != nil {
			var cancelFunc context.CancelFunc
			ctx, cancelFunc = context.WithTimeout(ctx, g.repository.Spec.Timeout.Duration)
			defer cancelFunc()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("unable to create request(GET): %q", url)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("unable to download file: %q", url)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad status: %q url: %q", resp.Status, url)
		}
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			return err
		}
		return nil
	}
}
