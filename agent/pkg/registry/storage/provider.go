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

package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"k3f.io/kubeforce/agent/pkg/config"
)

// RESTStorageProvider is a factory type for REST storage.
type RESTStorageProvider interface {
	GroupName() string
	NewRESTStorage(req *RESTStorageRequest) (genericapiserver.APIGroupInfo, error)
}

// RESTStorageRequest is a request parameters to create a new RESTStorage.
type RESTStorageRequest struct {
	Scheme            *runtime.Scheme
	ParameterCodec    runtime.ParameterCodec
	Codecs            serializer.CodecFactory
	RestOptionsGetter generic.RESTOptionsGetter
	Config            config.ConfigSpec
}
