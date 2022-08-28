/*
Copyright 2021

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

package latest

import (
	"k3f.io/kubeforce/agent/pkg/config"
	"k3f.io/kubeforce/agent/pkg/config/install"
	"k3f.io/kubeforce/agent/pkg/config/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
)

// Version is the string that represents the current external default version.
const Version = "v1alpha1"

var (
	// Codec is a Serializer for group config.agent.kubeforce.io
	Codec runtime.Codec
	// Scheme is a Scheme for group config.agent.kubeforce.io
	Scheme *runtime.Scheme
)

func init() {
	Scheme = runtime.NewScheme()
	install.Install(Scheme)
	options := json.SerializerOptions{Yaml: true, Strict: true}
	yamlSerializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, Scheme, Scheme, options)
	Codec = versioning.NewDefaultingCodecForScheme(
		Scheme,
		yamlSerializer,
		yamlSerializer,
		v1alpha1.SchemeGroupVersion,
		config.SchemeGroupVersion,
	)
}
