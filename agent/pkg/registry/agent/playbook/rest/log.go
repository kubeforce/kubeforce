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
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	"k3f.io/kubeforce/agent/pkg/apis/agent"
	"k3f.io/kubeforce/agent/pkg/apis/agent/validation"
)

// LogREST implements the log endpoint for a Playbook resource.
type LogREST struct {
	PlaybookPath string
	Store        *genericregistry.Store
}

// Destroy cleans up its resources on shutdown.
func (r *LogREST) Destroy() {
}

// LogREST implements GetterWithOptions.
var _ = rest.GetterWithOptions(&LogREST{})

// New creates a new Playbook log options object.
func (r *LogREST) New() runtime.Object {
	return &agent.Playbook{}
}

// ProducesMIMETypes returns a list of the MIME types the specified HTTP verb (GET, POST, DELETE,
// PATCH) can respond with.
func (r *LogREST) ProducesMIMETypes(verb string) []string {
	// Since the default list does not include "plain/text", we need to
	// explicitly override ProducesMIMETypes, so that it gets added to
	// the "produces" section for playbooks/{name}/log
	return []string{
		"text/plain",
	}
}

// ProducesObject returns an object the specified HTTP verb respond with. It will overwrite storage object if
// it is not nil. Only the type of the return object matters, the value will be ignored.
func (r *LogREST) ProducesObject(verb string) interface{} {
	return ""
}

// Get retrieves a runtime.Object that will stream the contents of the playbook log.
func (r *LogREST) Get(ctx context.Context, name string, opts runtime.Object) (runtime.Object, error) {
	logOpts, ok := opts.(*agent.PlaybookLogOptions)
	if !ok {
		return nil, fmt.Errorf("invalid options object: %#v", opts)
	}

	if errs := validation.ValidatePlaybookLogOptions(logOpts); len(errs) > 0 {
		return nil, apierrors.NewInvalid(agent.Kind("PlaybookLogOptions"), name, errs)
	}
	filePath, err := r.getLogFilePath(name, logOpts)
	if err != nil {
		return nil, err
	}

	return &FileStreamer{
		Path:        filePath,
		ContentType: "text/plain",
		Flush:       logOpts.Follow,
	}, nil
}

func (r *LogREST) getLogFilePath(name string, opts *agent.PlaybookLogOptions) (string, error) {
	dir := filepath.Join(r.PlaybookPath, name, "logs")
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.Errorf("unable to find logs directory")
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", errors.Wrap(err, "unable to read logs directory")
	}
	if len(files) == 0 {
		return "", errors.Wrap(err, "unable to find log files")
	}
	if opts.Previous {
		if len(files) < 2 {
			return "", errors.Wrap(err, "unable to find previous log file")
		}
		return filepath.Join(r.PlaybookPath, name, "logs", files[1].Name()), nil
	}
	return filepath.Join(r.PlaybookPath, name, "logs", files[0].Name()), nil
}

// NewGetOptions creates a new options object.
func (r *LogREST) NewGetOptions() (runtime.Object, bool, string) {
	return &agent.PlaybookLogOptions{}, false, ""
}

// OverrideMetricsVerb override the GET verb to CONNECT for playbook log resource.
func (r *LogREST) OverrideMetricsVerb(oldVerb string) (newVerb string) {
	newVerb = oldVerb

	if oldVerb == "GET" {
		newVerb = "CONNECT"
	}

	return
}
