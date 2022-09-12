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

package apiserver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/klog/v2"
)

const (
	paramMode      = "mode"
	paramPath      = "path"
	paramCreateDir = "createDir"
)

// NewUploadHandler creates a new handler for uploading the files to the host
func NewUploadHandler() *UploadHandler {
	return &UploadHandler{
		// Maximum upload of 512 MB files
		maxMemory: 512 << 20,
	}
}

// UploadHandler is a handler for uploading the files to the host
type UploadHandler struct {
	maxMemory int64
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		responsewriters.ErrorNegotiated(
			apierrors.NewInternalError(fmt.Errorf("method not allowed %q", req.Method)),
			Codecs, schema.GroupVersion{}, w, req,
		)
		return
	}
	if err := h.uploadFile(req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		responsewriters.ErrorNegotiated(
			apierrors.NewInternalError(err),
			Codecs, schema.GroupVersion{}, w, req,
		)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *UploadHandler) uploadFile(r *http.Request) error {
	err := r.ParseMultipartForm(h.maxMemory)
	if err != nil {
		return errors.Errorf("unable to parse a request body as multipart/form-data")
	}
	values := r.URL.Query()

	if !values.Has(paramPath) {
		return errors.Errorf("path is not defined")
	}
	targetPath := values.Get(paramPath)
	fileMode := os.FileMode(0o600)

	if values.Has(paramMode) {
		mode := values.Get(paramMode)
		val, err := strconv.ParseUint(mode, 8, 32)
		if err != nil {
			return errors.Errorf("unable parse file mode %s", mode)
		}
		fileMode = os.FileMode(val)
	}

	file, handler, err := r.FormFile("data")
	if err != nil {
		return errors.Wrap(err, "unable to parse form")
	}
	defer file.Close()
	klog.Infof("saving file %q, size: %+v, header: %+v", targetPath, handler.Size, handler.Header)

	if values.Has(paramCreateDir) && isTrue(values.Get(paramCreateDir)) {
		targetDir := filepath.Dir(targetPath)
		dirMode := os.FileMode(0o700)
		if err := os.MkdirAll(targetDir, dirMode); err != nil {
			return errors.Errorf("unable to create a directory %q", targetDir)
		}
	}

	dst, err := os.OpenFile(targetPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fileMode)
	defer dst.Close()
	if err != nil {
		return errors.Errorf("unable to create a file %q", targetPath)
	}

	// Copy the uploaded file to the created file on the filesystem
	if _, err := io.Copy(dst, file); err != nil {
		return errors.WithStack(err)
	}
	klog.Infof("the file has been uploaded %q", targetPath)
	return nil
}

func isTrue(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}
	return b
}
