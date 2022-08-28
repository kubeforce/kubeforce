package util

import (
	"context"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FindOwnerReference returns the OwnerReference object owning the current resource
func FindOwnerReference(obj metav1.ObjectMeta, kind, group string) (*metav1.OwnerReference, error) {
	for _, ref := range obj.OwnerReferences {
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, err
		}
		if ref.Kind == kind && gv.Group == group {
			return &ref, nil
		}
	}
	return nil, nil
}

// GetOwnerKubeforceMachinePool returns the KubeforceMachinePool object owning the current resource.
func GetOwnerKubeforceMachinePool(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*infrav1.KubeforceMachinePool, error) {
	for _, ref := range obj.OwnerReferences {
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, err
		}
		if ref.Kind == "KubeforceMachinePool" && gv.Group == infrav1.GroupVersion.Group {
			return GetKubeforceMachinePoolByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// GetKubeforceMachinePoolByName finds and return a KubeforceMachinePool object using the specified params.
func GetKubeforceMachinePoolByName(ctx context.Context, c client.Client, namespace, name string) (*infrav1.KubeforceMachinePool, error) {
	m := &infrav1.KubeforceMachinePool{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}
