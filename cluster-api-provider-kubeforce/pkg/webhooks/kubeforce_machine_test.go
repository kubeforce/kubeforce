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

package webhooks

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
)

func TestKubeforceMachineValidateCreate(t *testing.T) {
	pbTmpl := &infrav1.PlaybookTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       "PlaybookTemplate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "init-dev",
			Namespace: "default",
		},
		Spec: infrav1.PlaybookTemplateSpec{
			Spec: infrav1.RemotePlaybookSpec{
				Files: map[string]string{
					"playbook.yaml": "#empty",
				},
				Entrypoint: "playbook.yaml",
			},
		},
	}
	pbDpTmpl := &infrav1.PlaybookDeploymentTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       "PlaybookDeploymentTemplate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "boot-dev",
			Namespace: "default",
		},
		Spec: infrav1.PlaybookDeploymentTemplateSpec{
			Template: infrav1.PlaybookTemplateSpec{
				Spec: infrav1.RemotePlaybookSpec{
					Files: map[string]string{
						"playbook.yaml": "#empty",
					},
					Entrypoint: "playbook.yaml",
				},
			},
		},
	}

	getKubeforceMachine := func(fn func(addr *infrav1.KubeforceMachine)) infrav1.KubeforceMachine {
		ma := infrav1.KubeforceMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
			},
			Spec: infrav1.KubeforceMachineSpec{
				PlaybookTemplates: &infrav1.PlaybookTemplates{
					References: map[string]*infrav1.TemplateReference{
						"init": {
							Kind:       "PlaybookTemplate",
							Namespace:  "default",
							Name:       "init-dev",
							APIVersion: infrav1.GroupVersion.String(),
							Priority:   10,
							Type:       "install",
						},
						"boot": {
							Kind:       "PlaybookDeploymentTemplate",
							Namespace:  "default",
							Name:       "boot-dev",
							APIVersion: infrav1.GroupVersion.String(),
							Priority:   20,
							Type:       "install",
						},
					},
				},
			},
		}
		if fn != nil {
			fn(&ma)
		}
		return ma
	}

	tests := []struct {
		name      string
		kfMachine infrav1.KubeforceMachine
		extraObjs []client.Object
		expectErr bool
	}{
		{
			name:      "a valid KubeforceMachine should be accepted",
			kfMachine: getKubeforceMachine(nil),
			extraObjs: []client.Object{pbTmpl, pbDpTmpl},
			expectErr: false,
		},
		{
			name: "a priority that is negative should be rejected",
			kfMachine: getKubeforceMachine(func(ma *infrav1.KubeforceMachine) {
				ma.Spec.PlaybookTemplates.References["init"].Priority = -1
			}),
			extraObjs: []client.Object{pbTmpl, pbDpTmpl},
			expectErr: true,
		},
		{
			name: "a type that is not supported should be rejected",
			kfMachine: getKubeforceMachine(func(ma *infrav1.KubeforceMachine) {
				ma.Spec.PlaybookTemplates.References["init"].Type = "unsupported"
			}),
			extraObjs: []client.Object{pbTmpl, pbDpTmpl},
			expectErr: true,
		},
		{
			name: "a apiVersion that is not supported should be rejected",
			kfMachine: getKubeforceMachine(func(ma *infrav1.KubeforceMachine) {
				ma.Spec.PlaybookTemplates.References["init"].APIVersion = "v1"
			}),
			extraObjs: []client.Object{pbTmpl, pbDpTmpl},
			expectErr: true,
		},
		{
			name: "a kind that is not supported should be rejected",
			kfMachine: getKubeforceMachine(func(ma *infrav1.KubeforceMachine) {
				ma.Spec.PlaybookTemplates.References["init"].Kind = "Playbook"
			}),
			extraObjs: []client.Object{pbTmpl, pbDpTmpl},
			expectErr: true,
		},
		{
			name: "a template reference that does not match the template should be rejected",
			kfMachine: getKubeforceMachine(func(ma *infrav1.KubeforceMachine) {
				ma.Spec.PlaybookTemplates.References["init"].Name = "nothing"
			}),
			extraObjs: []client.Object{pbTmpl, pbDpTmpl},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())
			wh := KubeforceMachine{
				Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.extraObjs...).Build(),
			}
			if tt.expectErr {
				g.Expect(wh.validate(context.Background(), &tt.kfMachine)).NotTo(Succeed())
			} else {
				g.Expect(wh.validate(context.Background(), &tt.kfMachine)).To(Succeed())
			}
		})
	}
}

func TestKubeforceMachineValidateUpdate(t *testing.T) {
	getKubeforceMachine := func(fn func(addr *infrav1.KubeforceMachine)) infrav1.KubeforceMachine {
		ma := infrav1.KubeforceMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
			},
			Spec: infrav1.KubeforceMachineSpec{},
			Status: infrav1.KubeforceMachineStatus{
				AgentRef: &corev1.LocalObjectReference{
					Name: "default-agent",
				},
			},
		}
		if fn != nil {
			fn(&ma)
		}
		return ma
	}

	tests := []struct {
		name         string
		oldKfMachine infrav1.KubeforceMachine
		newKfMachine infrav1.KubeforceMachine
		extraObjs    []client.Object
		expectErr    bool
	}{
		{
			name:         "should accept objects with identical AgentRef",
			oldKfMachine: getKubeforceMachine(nil),
			newKfMachine: getKubeforceMachine(nil),
			expectErr:    false,
		},
		{
			name:         "should reject objects with different spec",
			oldKfMachine: getKubeforceMachine(nil),
			newKfMachine: getKubeforceMachine(func(ma *infrav1.KubeforceMachine) {
				ma.Status.AgentRef.Name = ""
			}),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())
			wh := KubeforceMachine{
				Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.extraObjs...).Build(),
			}
			if tt.expectErr {
				g.Expect(wh.ValidateUpdate(context.Background(), &tt.oldKfMachine, &tt.newKfMachine)).NotTo(Succeed())
			} else {
				g.Expect(wh.ValidateUpdate(context.Background(), &tt.oldKfMachine, &tt.newKfMachine)).To(Succeed())
			}
		})
	}
}
