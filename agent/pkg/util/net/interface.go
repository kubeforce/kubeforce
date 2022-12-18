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

package net

import (
	"fmt"
	"net"

	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"
)

// ChooseHostInterfaceByIP looks at all system interfaces, trying to find the one with the specified IP address.
func ChooseHostInterfaceByIP(ip net.IP) (*net.Interface, error) {
	intfs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	if len(intfs) == 0 {
		return nil, fmt.Errorf("no interfaces found on host")
	}
	for _, intf := range intfs {
		addrs, err := intf.Addrs()
		if err != nil {
			return nil, err
		}
		if len(addrs) == 0 {
			klog.V(4).Infof("Skipping: no addresses on interface %q", intf.Name)
			continue
		}
		for _, addr := range addrs {
			parsedIP, _, err := netutils.ParseCIDRSloppy(addr.String())
			if err != nil {
				return nil, fmt.Errorf("unable to parse CIDR for interface %q: %s", intf.Name, err)
			}
			if ip.Equal(parsedIP) {
				return &intf, nil
			}
		}
	}
	return nil, fmt.Errorf("no acceptable interface with ip address %s", ip)
}
