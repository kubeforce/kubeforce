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

// CommandTask takes the command name followed by a list of space-delimited arguments.
type CommandTask struct {
	Task   `json:",inline"`
	Params CommandTaskParams `json:"copy"`
}

// CommandTaskParams is parameters for command module
// see: https://docs.ansible.com/ansible/latest/collections/ansible/builtin/command_module.html
type CommandTaskParams struct {
	// Cmd is the command to run.
	Cmd string `json:"cmd,omitempty"`
	// Chdir changes into this directory before running the command.
	Chdir string `json:"chdir,omitempty"`
	// Argv passes the command as a list rather than a string.
	// Use argv to avoid quoting values that would otherwise be interpreted incorrectly (for example "user name").
	// Only the string (free form) or the list (argv) form can be provided, not both. One or the other must be provided.
	Argv []string `json:"argv,omitempty"`
	// Creates is a filename or (since 2.0) glob pattern. If a matching file already exists, this step will not be run.
	// This is checked before removes is checked.
	Creates string `json:"creates,omitempty"`
}
