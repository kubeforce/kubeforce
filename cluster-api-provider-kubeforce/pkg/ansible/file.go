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

// FileTask manages files and file properties.
type FileTask struct {
	Task   `json:",inline"`
	Params FileTaskParams `json:"file"`
}

// FileState describes the state for file module.
type FileState string

// Values that can be set for FileState.
const (
	FileStateAbsent    FileState = "absent"
	FileStateDirectory FileState = "directory"
	FileStateFile      FileState = "file"
	FileStateHard      FileState = "hard"
	FileStateLink      FileState = "link"
	FileStateTouch     FileState = "touch"
)

// FileTaskParams is parameters for file module
// see: https://docs.ansible.com/ansible/latest/collections/ansible/builtin/file_module.html
type FileTaskParams struct {
	// Dest is path to the file being managed..
	Dest string `json:"dest,omitempty"`
	// Owner is a name of the user that should own the filesystem object, as would be fed to chown.
	Owner string `json:"owner,omitempty"`
	// Group is a name of the group that should own the filesystem object, as would be fed to chown.
	Group string `json:"group,omitempty"`
	// Mode is the permissions the resulting filesystem object should have.
	Mode string `json:"mode,omitempty"`
	// State is type of operation
	// If absent, directories will be recursively deleted, and files or symlinks will be unlinked. In the case of a directory, if diff is declared, you will see the files and folders deleted listed under path_contents. Note that absent will not cause file to fail if the path does not exist as the state did not change.
	// If directory, all intermediate subdirectories will be created if they do not exist. Since Ansible 1.7 they will be created with the supplied permissions.
	// If file, with no other options, returns the current state of path.
	// If file, even with other options (such as mode), the file will be modified if it exists but will NOT be created if it does not exist. Set to touch or use the ansible.builtin.copy or ansible.builtin.template module if you want to create the file if it does not exist.
	// If hard, the hard link will be created or changed.
	// If link, the symbolic link will be created or changed.
	// If touch (new in 1.4), an empty file will be created if the file does not exist, while an existing file or directory will receive updated file access and modification times (similar to the way touch works from the command line).
	State FileState `json:"state,omitempty"`
}
