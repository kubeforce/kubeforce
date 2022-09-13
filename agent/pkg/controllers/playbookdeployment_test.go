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

package controllers

import (
	"strings"
	"testing"
	"time"

	clientset "k3f.io/kubeforce/agent/pkg/generated/clientset/versioned"

	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/gomega"
	"k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSuccessfulPlaybookDeployment(t *testing.T) {
	g := NewGomegaWithT(t)
	plName := "test-playbook"
	t.Run("run the successful playbookDeployment", func(t *testing.T) {
		p := &v1alpha1.PlaybookDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: plName,
			},
			Spec: v1alpha1.PlaybookDeploymentSpec{
				Template: v1alpha1.PlaybookTemplateSpec{
					Spec: v1alpha1.PlaybookSpec{
						Files: map[string]string{
							"site.yml": simpePlaybook,
						},
						Entrypoint: "site.yml",
					},
				},
			},
		}
		g.Expect(k8sClient.Create(ctx, p)).Should(Succeed())

		pdKey := types.NamespacedName{Name: plName}
		createdPlaybookDep := &v1alpha1.PlaybookDeployment{}

		g.Eventually(func() bool {
			err := k8sClient.Get(ctx, pdKey, createdPlaybookDep)
			if err != nil {
				return false
			}
			return createdPlaybookDep.Status.Phase == v1alpha1.PlaybookDeploymentSucceeded
		}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		g.Expect(createdPlaybookDep.Status.Phase).Should(Equal(v1alpha1.PlaybookDeploymentSucceeded))
		cs, err := clientset.NewForConfig(restcfg)
		g.Expect(err).Should(Succeed())
		list, err := cs.AgentV1alpha1().Playbooks().List(ctx, metav1.ListOptions{})
		g.Expect(err).Should(Succeed())
		count := 0
		var playbook *v1alpha1.Playbook
		for i := range list.Items {
			item := &list.Items[i]
			if metav1.IsControlledBy(item, p) {
				count++
				playbook = item
			}
		}
		g.Expect(count).Should(Equal(1))
		res := cs.AgentV1alpha1().Playbooks().GetLogs(playbook.Name, &v1alpha1.PlaybookLogOptions{}).Do(ctx)
		g.Expect(res.Warnings()).Should(BeEmpty())
		raw, err := res.Raw()
		g.Expect(err).Should(Succeed())
		g.Expect(strings.Contains(string(raw), "This message should be in the log file")).Should(BeTrue())
	})
}
