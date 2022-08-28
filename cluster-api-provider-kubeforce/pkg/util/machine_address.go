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
