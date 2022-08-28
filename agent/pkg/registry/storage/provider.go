package storage

import (
	"k3f.io/kubeforce/agent/pkg/config"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

// RESTStorageProvider is a factory type for REST storage.
type RESTStorageProvider interface {
	GroupName() string
	NewRESTStorage(req *RESTStorageRequest) (genericapiserver.APIGroupInfo, error)
}

// RESTStorageRequest is a request parameters to create a new RESTStorage
type RESTStorageRequest struct {
	Scheme            *runtime.Scheme
	ParameterCodec    runtime.ParameterCodec
	Codecs            serializer.CodecFactory
	RestOptionsGetter generic.RESTOptionsGetter
	Config            config.ConfigSpec
}
