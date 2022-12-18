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

package rest

import (
	"context"
	"net"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiutilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/registry/rest"

	"k3f.io/kubeforce/agent/pkg/apis/agent"
	utilnet "k3f.io/kubeforce/agent/pkg/util/net"
)

var startTime = time.Now()

// SysInfoREST implements the sysinfo endpoint.
type SysInfoREST struct {
}

// Destroy cleans up its resources on shutdown.
func (r *SysInfoREST) Destroy() {
}

var _ rest.Getter = &SysInfoREST{}
var _ rest.Scoper = &SysInfoREST{}

// New creates a new Playbook log options object.
func (r *SysInfoREST) New() runtime.Object {
	return &agent.SysInfo{}
}

// NamespaceScoped returns false it means this resource is global.
func (r *SysInfoREST) NamespaceScoped() bool {
	return false
}

// Get retrieves a runtime.Object that will stream the contents of the playbook log.
func (r *SysInfoREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	ip, err := apiutilnet.ChooseHostInterface()
	if err != nil {
		return nil, err
	}
	interfaceByIP, err := utilnet.ChooseHostInterfaceByIP(ip)
	if err != nil {
		return nil, err
	}
	interfaces, err := r.getInterfaces()
	if err != nil {
		return nil, err
	}
	return &agent.SysInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(startTime),
		},
		Spec: agent.SysInfoSpec{
			Network: agent.Network{
				Hostname:             hostname,
				DefaultIPAddress:     ip.String(),
				DefaultInterfaceName: interfaceByIP.Name,
				Interfaces:           interfaces,
			},
		},
	}, nil
}

func (r *SysInfoREST) getInterfaces() ([]agent.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := make([]agent.Interface, 0, len(interfaces))
	for _, intf := range interfaces {
		agentInterface, err := r.toAgentInterface(intf)
		if err != nil {
			return nil, err
		}
		result = append(result, *agentInterface)
	}
	return result, nil
}

func (r *SysInfoREST) toAgentInterface(intf net.Interface) (*agent.Interface, error) {
	netAddrs, err := intf.Addrs()
	if err != nil {
		return nil, err
	}
	addrs := make([]string, 0, len(netAddrs))
	for _, addr := range netAddrs {
		addrs = append(addrs, addr.String())
	}
	var flags []string
	intfFlags := intf.Flags.String()
	if intfFlags != "0" && intfFlags != "" {
		flags = strings.Split(intfFlags, "|")
	}

	return &agent.Interface{
		Name:      intf.Name,
		Addresses: addrs,
		Flags:     flags,
		Status:    r.toAgentInterfaceStatus(intf),
	}, nil
}

func (r *SysInfoREST) toAgentInterfaceStatus(intf net.Interface) agent.InterfaceStatus {
	if intf.Flags&net.FlagUp != 0 {
		return agent.InterfaceStatusUP
	}
	return agent.InterfaceStatusDOWN
}
