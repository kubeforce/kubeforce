/*
Copyright The Kubeforce Authors.

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

package fake

import (
	"k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
	"k8s.io/client-go/rest"
)

// GetLogs constructs a request for getting the logs for a playbook
func (c *FakePlaybooks) GetLogs(name string, opts *v1alpha1.PlaybookLogOptions) *rest.Request {
	//TODO implement me
	panic("implement me")
}
