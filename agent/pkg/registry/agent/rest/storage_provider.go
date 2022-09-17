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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"k3f.io/kubeforce/agent/pkg/apis/agent"
	"k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
	"k3f.io/kubeforce/agent/pkg/config"
	"k3f.io/kubeforce/agent/pkg/registry/agent/playbook"
	playbookdeployment "k3f.io/kubeforce/agent/pkg/registry/agent/playbookdepoyment"
	"k3f.io/kubeforce/agent/pkg/registry/agent/sysinfo"
	"k3f.io/kubeforce/agent/pkg/registry/storage"
)

type StorageProvider struct{}

var _ storage.RESTStorageProvider = StorageProvider{}

func (p StorageProvider) NewRESTStorage(req *storage.RESTStorageRequest) (genericapiserver.APIGroupInfo, error) {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(agent.GroupName, req.Scheme, req.ParameterCodec, req.Codecs)

	storageMap, err := p.v1alpha1Storage(req.Scheme, req.RestOptionsGetter, req.Config)
	if err != nil {
		return genericapiserver.APIGroupInfo{}, err
	}
	apiGroupInfo.VersionedResourcesStorageMap[v1alpha1.SchemeGroupVersion.Version] = storageMap

	return apiGroupInfo, nil
}

func (p StorageProvider) v1alpha1Storage(scheme *runtime.Scheme, restOptionsGetter generic.RESTOptionsGetter, cfg config.ConfigSpec) (map[string]rest.Storage, error) {
	storageMap := map[string]rest.Storage{}
	// playbooks
	restStorage, statusStorage, logStorage, err := playbook.NewREST(scheme, restOptionsGetter, cfg)
	if err != nil {
		return nil, err
	}
	storageMap["playbooks"] = restStorage
	storageMap["playbooks/status"] = statusStorage
	storageMap["playbooks/log"] = logStorage

	// playbookdeployments
	pbdRestStorage, pbdStatusStorage, err := playbookdeployment.NewREST(scheme, restOptionsGetter)
	if err != nil {
		return nil, err
	}
	storageMap[playbookdeployment.GroupResource.Resource] = pbdRestStorage
	storageMap[playbookdeployment.GroupResource.Resource+"/status"] = pbdStatusStorage

	// sysinfos
	sysInfoREST, err := sysinfo.NewREST(scheme)
	if err != nil {
		return nil, err
	}
	storageMap[sysinfo.GroupResource.Resource] = sysInfoREST

	return storageMap, err
}

func (p StorageProvider) GroupName() string {
	return agent.GroupName
}
