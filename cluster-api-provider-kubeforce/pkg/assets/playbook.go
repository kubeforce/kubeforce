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

package assets

import (
	"embed"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/ansible"
	"sigs.k8s.io/yaml"
)

var (
	//go:embed playbooks
	playbooks embed.FS
)

// PlaybookName is a type for predefined playbook names.
type PlaybookName string

const (
	// PlaybookInstaller is a playbook to install containerd and kubelet on the node
	PlaybookInstaller    PlaybookName = "installer"
	PlaybookCleaner      PlaybookName = "cleaner"
	PlaybookLoadbalancer PlaybookName = "loadbalancer"
)

func GetPlaybook(name PlaybookName, vars map[string]interface{}) (*ansible.Playbook, error) {
	playbook := ansible.NewPlaybook("playbook.yaml")
	if err := addFiles("", path.Join("playbooks", string(name)), playbook); err != nil {
		return nil, err
	}
	if vars != nil {
		varsData, err := yaml.Marshal(vars)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to marshal for playbook %s", name)
		}
		playbook.Files["variables.yaml"] = string(varsData)
	}
	return playbook, nil
}

func addFiles(dstDir, srcDir string, playbook *ansible.Playbook) error {
	entries, err := playbooks.ReadDir(srcDir)
	if err != nil {
		return errors.Wrapf(err, "unable to read dir %q", srcDir)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if err := addFiles(filepath.Join(dstDir, entry.Name()), filepath.Join(srcDir, entry.Name()), playbook); err != nil {
				return errors.WithStack(err)
			}
			continue
		}
		content, err := playbooks.ReadFile(filepath.Join(srcDir, entry.Name()))
		if err != nil {
			return errors.WithStack(err)
		}
		dstFilename := filepath.Join(dstDir, entry.Name())
		playbook.Files[dstFilename] = string(content)
	}
	return nil
}
