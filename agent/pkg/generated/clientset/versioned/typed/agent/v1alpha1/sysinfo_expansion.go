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
	"context"

	"k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
)

// The SysInfoExpansion has methods to work with SysInfo resources.
// This interface is manually added.
type SysInfoExpansion interface {
	Get(ctx context.Context) (*v1alpha1.SysInfo, error)
}

// Get constructs a request for getting the system information from the host
func (c *sysInfos) Get(ctx context.Context) (*v1alpha1.SysInfo, error) {
	result := &v1alpha1.SysInfo{}
	err := c.client.Get().
		Resource("sysinfos").
		Name("local").
		Do(ctx).
		Into(result)
	return result, err
}
