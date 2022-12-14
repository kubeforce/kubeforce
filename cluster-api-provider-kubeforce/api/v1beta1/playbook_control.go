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

package v1beta1

import (
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:object:generate:=false

// PlaybookControlObject interface defines methods that an object should implement in order to manage playbooks.
type PlaybookControlObject interface {
	client.Object
	GetAgent() types.NamespacedName
	GetTemplates() *PlaybookTemplates
	GetConditions() clusterv1.Conditions
	SetConditions(conditions clusterv1.Conditions)
	GetPlaybookConditions() PlaybookConditions
	SetPlaybookConditions(PlaybookConditions)
}
