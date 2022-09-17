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
	"encoding/json"

	"github.com/pkg/errors"
)

// UserData describes user-data in the cloud-init configuration.
type UserData struct {
	// WriteFile is the list of files to be written to the host
	WriteFiles []WriteFile `json:"write_files,omitempty"`
	// RunCmd is a list of commands.
	RunCmd []Cmd `json:"runcmd,omitempty"`
}

// WriteFile defines the input for generating write_files in cloud-init.
type WriteFile struct {
	// Path specifies the full path on disk where to store the file.
	Path string `json:"path"`

	// Owner specifies the ownership of the file, format: "root:root".
	// +optional
	Owner string `json:"owner,omitempty"`

	// Permissions specifies the permissions to assign to the file.
	// +optional
	Permissions string `json:"permissions,omitempty"`

	// Encoding specifies the encoding of the file contents.
	// +optional
	Encoding string `json:"encoding,omitempty"`

	// Content is the actual content of the file.
	// +optional
	Content string `json:"content,omitempty"`
}

// Cmd defines a cloud-init command.
// It can be either a list or a string.
type Cmd struct {
	IsList bool
	Cmd    string
	List   []string
}

// UnmarshalJSON deserialize JSON to the object.
func (c *Cmd) UnmarshalJSON(data []byte) error {
	// First, try to decode the input as a list
	var s1 []string
	if err := json.Unmarshal(data, &s1); err != nil {
		if _, ok := err.(*json.UnmarshalTypeError); !ok {
			return errors.WithStack(err)
		}
	} else {
		c.Cmd = ""
		c.IsList = true
		c.List = s1
		return nil
	}

	// If it's not a list, it must be a string
	var s2 string
	if err := json.Unmarshal(data, &s2); err != nil {
		return errors.WithStack(err)
	}

	c.Cmd = s2
	c.IsList = false
	c.List = nil
	return nil
}

// MarshalJSON serialize the object to JSON format.
func (c *Cmd) MarshalJSON() ([]byte, error) {
	if c.IsList {
		return json.Marshal(c.List)
	}
	return json.Marshal(c.Cmd)
}
