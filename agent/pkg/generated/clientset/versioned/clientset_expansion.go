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

package versioned

import (
	"bytes"
	"context"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
)

// Uninstall uninstalls the agent from the host
func (c *Clientset) Uninstall(ctx context.Context) error {
	return c.RESTClient().Delete().
		AbsPath("uninstall").
		Do(ctx).
		Error()
}

// UploadData uploads content to the host and saves it as a file
func (c *Clientset) UploadData(ctx context.Context, targetPath string, data []byte, mode *os.FileMode) error {
	request := c.RESTClient().
		Post().
		AbsPath("upload").
		Param("path", targetPath)
	if mode != nil {
		request.Param("mode", strconv.FormatInt(int64(*mode), 8))
	}

	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)
	part, err := w.CreateFormFile("data", filepath.Base(targetPath))
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	if err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return errors.Wrap(err, "unable to close multipart writer")
	}
	request.SetHeader("Content-Type", w.FormDataContentType())
	request.Body(buf)
	return request.Do(ctx).Error()
}
