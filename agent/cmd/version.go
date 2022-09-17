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

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"k3f.io/kubeforce/agent/pkg/version"
)

var (
	versionExample = `
# Print the agent version information
agent version`
)

// NewCmdVersion returns a cobra command for fetching versions.
func NewCmdVersion() *cobra.Command {
	o := version.NewOptions()
	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Print the agent version information",
		Long:    `Print the agent version information.`,
		Example: versionExample,
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringVarP(&o.Output, "output", "o", o.Output, "One of 'yaml' or 'json'.")
	return cmd
}

func init() {
	rootCmd.AddCommand(NewCmdVersion())
}
