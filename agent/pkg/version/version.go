package version

import (
	"encoding/json"
	"fmt"

	"k8s.io/component-base/version"
	"sigs.k8s.io/yaml"
)

// Options is a struct to support version command
type Options struct {
	Output string
}

// NewOptions returns initialized Options
func NewOptions() *Options {
	return &Options{}
}

// Run executes version command
func (o *Options) Run() error {
	versionInfo := version.Get()

	switch o.Output {
	case "":
		fmt.Printf("Version: %s\n", versionInfo.GitVersion)
	case "yaml":
		marshalled, err := yaml.Marshal(&versionInfo)
		if err != nil {
			return err
		}
		fmt.Println(string(marshalled))
	case "json":
		marshalled, err := json.MarshalIndent(&versionInfo, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(marshalled))
	default:
		// There is a bug in the program if we hit this case.
		// However, we follow a policy of never panicking.
		return fmt.Errorf("VersionOptions were not validated: --output=%q should have been rejected", o.Output)
	}

	return nil
}
