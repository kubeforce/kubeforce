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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
	scheme "k3f.io/kubeforce/agent/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// PlaybooksGetter has a method to return a PlaybookInterface.
// A group's client should implement this interface.
type PlaybooksGetter interface {
	Playbooks() PlaybookInterface
}

// PlaybookInterface has methods to work with Playbook resources.
type PlaybookInterface interface {
	Create(ctx context.Context, playbook *v1alpha1.Playbook, opts v1.CreateOptions) (*v1alpha1.Playbook, error)
	Update(ctx context.Context, playbook *v1alpha1.Playbook, opts v1.UpdateOptions) (*v1alpha1.Playbook, error)
	UpdateStatus(ctx context.Context, playbook *v1alpha1.Playbook, opts v1.UpdateOptions) (*v1alpha1.Playbook, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.Playbook, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.PlaybookList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Playbook, err error)
	PlaybookExpansion
}

// playbooks implements PlaybookInterface
type playbooks struct {
	client rest.Interface
}

// newPlaybooks returns a Playbooks
func newPlaybooks(c *AgentV1alpha1Client) *playbooks {
	return &playbooks{
		client: c.RESTClient(),
	}
}

// Get takes name of the playbook, and returns the corresponding playbook object, and an error if there is any.
func (c *playbooks) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.Playbook, err error) {
	result = &v1alpha1.Playbook{}
	err = c.client.Get().
		Resource("playbooks").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Playbooks that match those selectors.
func (c *playbooks) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.PlaybookList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.PlaybookList{}
	err = c.client.Get().
		Resource("playbooks").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested playbooks.
func (c *playbooks) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("playbooks").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a playbook and creates it.  Returns the server's representation of the playbook, and an error, if there is any.
func (c *playbooks) Create(ctx context.Context, playbook *v1alpha1.Playbook, opts v1.CreateOptions) (result *v1alpha1.Playbook, err error) {
	result = &v1alpha1.Playbook{}
	err = c.client.Post().
		Resource("playbooks").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(playbook).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a playbook and updates it. Returns the server's representation of the playbook, and an error, if there is any.
func (c *playbooks) Update(ctx context.Context, playbook *v1alpha1.Playbook, opts v1.UpdateOptions) (result *v1alpha1.Playbook, err error) {
	result = &v1alpha1.Playbook{}
	err = c.client.Put().
		Resource("playbooks").
		Name(playbook.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(playbook).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *playbooks) UpdateStatus(ctx context.Context, playbook *v1alpha1.Playbook, opts v1.UpdateOptions) (result *v1alpha1.Playbook, err error) {
	result = &v1alpha1.Playbook{}
	err = c.client.Put().
		Resource("playbooks").
		Name(playbook.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(playbook).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the playbook and deletes it. Returns an error if one occurs.
func (c *playbooks) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("playbooks").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *playbooks) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("playbooks").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched playbook.
func (c *playbooks) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Playbook, err error) {
	result = &v1alpha1.Playbook{}
	err = c.client.Patch(pt).
		Resource("playbooks").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
