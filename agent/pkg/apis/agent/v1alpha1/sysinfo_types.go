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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +genclient:noStatus
// +genclient:onlyVerbs
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SysInfo provides the system information about this host
// +k8s:openapi-gen=true
type SysInfo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SysInfoSpec `json:"spec,omitempty"`
}

// SysInfoSpec defines the system information.
type SysInfoSpec struct {
	// Network is the network information
	// +optional
	Network Network `json:"network,omitempty"`
}

// Network defines the network information.
type Network struct {
	// Hostname is the current hostname
	Hostname string `json:"hostname"`
	// DefaultIPAddress is an ip address from default route
	DefaultIPAddress string `json:"defaultIPAddress,omitempty"`
	// DefaultInterfaceName is a network interface from default route
	DefaultInterfaceName string `json:"defaultInterfaceName,omitempty"`
	// Interfaces is the slice of network interfaces for this host
	// +optional
	Interfaces []Interface `json:"interfaces,omitempty"`
}

// Interface describes summary information about a network interface.
type Interface struct {
	Name      string          `json:"name"`
	Addresses []string        `json:"addresses"`
	Flags     []string        `json:"flags"`
	Status    InterfaceStatus `json:"status"`
}

// InterfaceStatus defines the status of network interface.
type InterfaceStatus string

// These are the valid phases of Playbook.
const (
	// InterfaceStatusUP means the network interface has been started.
	InterfaceStatusUP InterfaceStatus = "UP"
	// InterfaceStatusDOWN means the network interface has been stopped.
	InterfaceStatusDOWN InterfaceStatus = "DOWN"
)
