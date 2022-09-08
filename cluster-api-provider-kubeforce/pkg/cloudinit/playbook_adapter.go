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

package cloudinit

import (
	"path/filepath"
	"sort"
	"strings"

	kubeadmv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/ansible"
	"sigs.k8s.io/yaml"
)

// NewAnsibleAdapter creates a new ansible adapter
func NewAnsibleAdapter(kubeadmCfg kubeadmv1.KubeadmConfigSpec) *AnsibleAdapter {
	return &AnsibleAdapter{
		kubeadmCfg: kubeadmCfg,
	}
}

// AnsibleAdapter prepares Ansible playbook cloud-config file
type AnsibleAdapter struct {
	kubeadmCfg kubeadmv1.KubeadmConfigSpec
}

// ToPlaybook transform a cloud-config to a playbook
func (a *AnsibleAdapter) ToPlaybook(cloudConfig []byte) (*ansible.Playbook, error) {
	content, err := a.userDataToPlaybook(cloudConfig)
	if err != nil {
		return nil, err
	}

	playbook := &ansible.Playbook{
		Files: map[string]string{
			"playbook.yaml": string(content),
		},
		Entrypoint: "playbook.yaml",
	}
	return playbook, nil
}

func (a *AnsibleAdapter) userDataToPlaybook(cloudConfig []byte) ([]byte, error) {
	userData := &UserData{}
	if err := yaml.Unmarshal(cloudConfig, userData); err != nil {
		return nil, err
	}
	tasks := ansible.NewTasks("cloud-init")
	copyTasks := make([]*ansible.CopyTask, 0, len(userData.WriteFiles))
	dirMap := make(map[string]struct{})
	for _, wf := range userData.WriteFiles {
		task, err := a.writeFileToTask(wf)
		if err != nil {
			return nil, err
		}
		copyTasks = append(copyTasks, task)
		dirMap[filepath.Dir(task.Params.Dest)] = struct{}{}
	}
	dirs := make([]string, 0, len(dirMap))
	for dir := range dirMap {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		tasks.AddTasks(&ansible.FileTask{
			Params: ansible.FileTaskParams{
				Dest:  dir,
				State: ansible.FileStateDirectory,
			},
		})
	}
	for _, task := range copyTasks {
		tasks.AddTasks(task)
	}
	for _, cmd := range userData.RunCmd {
		tasks.AddTasks(a.cmdFileToTask(cmd))
	}
	content, err := ansible.Marshall(tasks)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func (a *AnsibleAdapter) writeFileToTask(f WriteFile) (*ansible.CopyTask, error) {
	task := &ansible.CopyTask{
		Params: ansible.CopyTaskParams{
			Dest: f.Path,
			Mode: f.Permissions,
		},
	}
	owner := strings.Split(strings.TrimSpace(f.Owner), ":")
	if len(owner) == 2 {
		task.Params.Owner = owner[0]
		task.Params.Group = owner[1]
	}
	encodings := fixEncoding(f.Encoding)
	content, err := fixContent(f.Content, encodings)
	if err != nil {
		return nil, err
	}
	task.Params.Content = content
	return task, nil
}

func (a *AnsibleAdapter) cmdFileToTask(c Cmd) ansible.TaskModule {
	if !c.IsList {
		return ansible.ShellTask{
			Params: ansible.ShellTaskParams{
				Cmd: c.Cmd,
			},
		}
	}
	return ansible.CommandTask{
		Params: ansible.CommandTaskParams{
			Argv: c.List,
		},
	}
}
