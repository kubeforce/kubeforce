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

package agent

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SysInfo provides the system information about this host
// +k8s:openapi-gen=true
type SysInfo struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec SysInfoSpec
}

// SysInfoSpec defines the system information
type SysInfoSpec struct {
	// Network is the network information
	// +optional
	Network Network
}

// Network defines the network information
type Network struct {
	// Hostname is the current hostname
	Hostname string
	// InternalIP is an ip address from default interface
	InternalIP string
	// Interfaces is the slice of network interfaces for this host
	// +optional
	Interfaces []Interface
}

type Interface struct {
	Name    string
	Address string
	Mac     string
	Status  InterfaceStatus
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
