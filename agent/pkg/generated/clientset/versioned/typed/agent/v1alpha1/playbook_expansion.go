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

package v1alpha1

import (
	"k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
	"k3f.io/kubeforce/agent/pkg/generated/clientset/versioned/scheme"
	restclient "k8s.io/client-go/rest"
)

// The PlaybookExpansion interface allows manually adding extra methods to the PlaybookInterface.
type PlaybookExpansion interface {
	GetLogs(name string, opts *v1alpha1.PlaybookLogOptions) *restclient.Request
}

// GetLogs constructs a request for getting the logs for a playbook
func (c *playbooks) GetLogs(name string, opts *v1alpha1.PlaybookLogOptions) *restclient.Request {
	return c.client.Get().Name(name).Resource("playbooks").SubResource("log").VersionedParams(opts, scheme.ParameterCodec)
}
