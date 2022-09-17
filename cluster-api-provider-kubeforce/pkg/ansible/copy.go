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

// CopyTask copies a file from the local or remote machine to a location on the remote machine.
type CopyTask struct {
	Task   `json:",inline"`
	Params CopyTaskParams `json:"copy"`
}

// CopyTaskParams is parameters for copy module
// see: https://docs.ansible.com/ansible/latest/collections/ansible/builtin/copy_module.html
type CopyTaskParams struct {
	// Dest is remote absolute path where the file should be copied to.
	Dest string `json:"dest,omitempty"`
	// Checksum is SHA1 checksum of the file being transferred.
	// Used to validate that the copy of the file was successful.
	// If this is not provided, ansible will use the local calculated checksum of the src file.
	Checksum string `json:"checksum,omitempty"`
	// Owner is a name of the user that should own the filesystem object, as would be fed to chown.
	Owner string `json:"owner,omitempty"`
	// Group is a name of the group that should own the filesystem object, as would be fed to chown.
	Group string `json:"group,omitempty"`
	// Mode is the permissions the resulting filesystem object should have.
	Mode string `json:"mode,omitempty"`
	// Content is the contents of a file when used instead of src
	Content string `json:"content,omitempty"`
}
