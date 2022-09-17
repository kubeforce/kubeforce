/*
Copyright 2020 The Kubeforce Authors.

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

package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/pkg/errors"
)

// CalcSHA256Sum calculates sha256 checksum for content.
func CalcSHA256Sum(content []byte) (string, error) {
	h := sha256.New()
	_, err := h.Write(content)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CalcSHA256ForObject calculates sha256 checksum for object.
func CalcSHA256ForObject(obj interface{}) (string, error) {
	jsonData, err := json.Marshal(obj)
	if err != nil {
		return "", errors.WithStack(err)
	}
	sum, err := CalcSHA256Sum(jsonData)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return sum, nil
}
