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

package ansible

// NewTasks creates a new ansible Tasks.
func NewTasks(name string) *Tasks {
	return &Tasks{
		Name:       name,
		Hosts:      "localhost",
		Connection: "local",
		Become:     true,
		VarFiles:   []string{"variables.yaml"},
		Tasks:      make([]TaskModule, 0),
	}
}

// Tasks describes an ansible tasks.
type Tasks struct {
	Name       string       `json:"name,omitempty"`
	Hosts      string       `json:"hosts"`
	Connection string       `json:"connection,omitempty"`
	Become     bool         `json:"become"`
	Tasks      []TaskModule `json:"tasks"`
	VarFiles   []string     `json:"vars_files,omitempty"`
}

// AddTasks adds tasks to ansible tasks module.
func (t *Tasks) AddTasks(tasks ...TaskModule) {
	t.Tasks = append(t.Tasks, tasks...)
}

// TaskModule describe the ansible task module.
type TaskModule interface{}

// Task describes the general parameters of the task.
type Task struct {
	Name string `json:"name,omitempty"`
}
