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

package util

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

func FindAddress(addresses clusterv1.MachineAddresses, addrType clusterv1.MachineAddressType) string {
	for _, address := range addresses {
		if address.Type == addrType {
			return address.Address
		}
	}
	return ""
}

func FindAddrWithPriority(addresses clusterv1.MachineAddresses, types ...clusterv1.MachineAddressType) string {
	for _, t := range types {
		addr := FindAddress(addresses, t)
		if addr != "" {
			return addr
		}
	}
	return ""
}

func GetAddresses(addresses clusterv1.MachineAddresses, types ...clusterv1.MachineAddressType) []string {
	result := make([]string, 0)
	for _, address := range addresses {
		for _, t := range types {
			if address.Type == t {
				result = append(result, address.Address)
			}
		}

	}
	return result
}
