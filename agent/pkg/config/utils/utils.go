/*
Copyright 2021

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

package utils

import (
	"io/ioutil"

	"k3f.io/kubeforce/agent/pkg/config"
	"k3f.io/kubeforce/agent/pkg/config/latest"
	"k3f.io/kubeforce/agent/pkg/config/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// LoadFromFile deserializes the contents from file into Config object
func LoadFromFile(filepath string) (*config.Config, error) {
	bytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	return Unmarshal(bytes)
}

// Unmarshal takes a byte slice and deserializes the contents into Config object.
func Unmarshal(data []byte) (*config.Config, error) {
	r := &config.Config{}
	// if there's no data in a file, return the default object instead of failing (DecodeInto reject empty input)
	if len(data) == 0 {
		return r, nil
	}
	decoded, _, err := latest.Codec.Decode(data, &schema.GroupVersionKind{Version: latest.Version, Kind: "Config"}, r)
	if err != nil {
		return nil, err
	}
	cfg := decoded.(*config.Config)
	err = validation.Validate(cfg).ToAggregate()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// Marshal serializes the Config to yaml.
func Marshal(r *config.Config) ([]byte, error) {
	return runtime.Encode(latest.Codec, r)
}
