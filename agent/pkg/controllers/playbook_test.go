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

	"k3f.io/kubeforce/agent/pkg/util/conditions"

	clientset "k3f.io/kubeforce/agent/pkg/generated/clientset/versioned"

	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/gomega"
	"k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var simpePlaybook = `
- hosts: all

  tasks:
    - name: simple-playbook 
      debug:
        msg: This message should be in the log file
`

var badPlaybook = `
- hosts: all

  tasks:
    - name: failed-playbook 
      shell:
        cmd: echo 'custom error message' && exit 1
`

func TestSuccessfulPlaybook(t *testing.T) {
	g := NewGomegaWithT(t)
	plName := "test-playbook"
	t.Run("run the successful playbook", func(t *testing.T) {
		p := &v1alpha1.Playbook{
			ObjectMeta: metav1.ObjectMeta{
				Name: plName,
			},
			Spec: v1alpha1.PlaybookSpec{
				Files: map[string]string{
					"site.yml": simpePlaybook,
				},
				Entrypoint: "site.yml",
			},
		}
		g.Expect(k8sClient.Create(ctx, p)).Should(Succeed())

		playbookKey := types.NamespacedName{Name: plName}
		createdPlaybook := &v1alpha1.Playbook{}

		g.Eventually(func() bool {
			err := k8sClient.Get(ctx, playbookKey, createdPlaybook)
			if err != nil {
				return false
			}
			if conditions.IsTrue(createdPlaybook, v1alpha1.PlaybookExecutionCondition) ||
				conditions.IsTrue(createdPlaybook, v1alpha1.PlaybookFailedCondition) {
				return true
			}
			return false
		}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		g.Expect(createdPlaybook.Status.Phase).Should(Equal(v1alpha1.PlaybookSucceeded))
		g.Expect(conditions.IsTrue(createdPlaybook, v1alpha1.PlaybookExecutionCondition)).Should(BeTrue())
		cs, err := clientset.NewForConfig(restcfg)
		g.Expect(err).Should(Succeed())
		res := cs.AgentV1alpha1().Playbooks().GetLogs(plName, &v1alpha1.PlaybookLogOptions{}).Do(ctx)
		g.Expect(res.Error()).Should(Succeed())
		g.Expect(res.Warnings()).Should(BeEmpty())
		raw, err := res.Raw()
		g.Expect(err).Should(Succeed())
		g.Expect(strings.Contains(string(raw), "This message should be in the log file")).Should(BeTrue())
	})
}

func TestFailedPlaybook(t *testing.T) {
	DefaultJobBackOff = time.Duration(0) // overwrite the default value for testing
	g := NewGomegaWithT(t)
	plName := "playbook"
	t.Run("run the bad playbook", func(t *testing.T) {
		p := &v1alpha1.Playbook{
			ObjectMeta: metav1.ObjectMeta{
				Name: plName,
			},
			Spec: v1alpha1.PlaybookSpec{
				Files: map[string]string{
					"site.yml": badPlaybook,
				},
				Entrypoint: "site.yml",
			},
		}
		g.Expect(k8sClient.Create(ctx, p)).Should(Succeed())

		playbookKey := types.NamespacedName{Name: plName}
		createdPlaybook := &v1alpha1.Playbook{}

		g.Eventually(func() bool {
			err := k8sClient.Get(ctx, playbookKey, createdPlaybook)
			if err != nil {
				return false
			}
			return conditions.IsTrue(createdPlaybook, v1alpha1.PlaybookExecutionCondition) ||
				conditions.IsTrue(createdPlaybook, v1alpha1.PlaybookFailedCondition)
		}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		g.Expect(createdPlaybook.Status.Phase).Should(Equal(v1alpha1.PlaybookFailed))
		g.Expect(createdPlaybook.Status.Failed).Should(Equal(int32(3)))
		g.Expect(conditions.IsTrue(createdPlaybook, v1alpha1.PlaybookExecutionCondition)).Should(BeFalse())
		g.Expect(conditions.IsTrue(createdPlaybook, v1alpha1.PlaybookFailedCondition)).Should(BeTrue())
		cs, err := clientset.NewForConfig(restcfg)
		g.Expect(err).Should(Succeed())
		res := cs.AgentV1alpha1().Playbooks().GetLogs(plName, &v1alpha1.PlaybookLogOptions{}).Do(ctx)
		g.Expect(res.Error()).Should(Succeed())
		g.Expect(res.Warnings()).Should(BeEmpty())
		raw, err := res.Raw()
		g.Expect(err).Should(Succeed())
		g.Expect(string(raw)).Should(ContainSubstring("custom error message"))
	})
}
