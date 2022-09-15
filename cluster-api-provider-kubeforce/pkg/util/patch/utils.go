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

// Package patch implements patch utilities.
package patch

import (
	"encoding/json"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HasChanges checks the patch data to determine if this object has changes or not.
func HasChanges(patchObj client.Patch, obj client.Object) (bool, error) {
	diff, err := patchObj.Data(obj)
	if err != nil {
		return false, errors.Wrapf(err, "failed to calculate patch data")
	}

	// Unmarshal patch data into a local map.
	patchDiff := map[string]interface{}{}
	if err := json.Unmarshal(diff, &patchDiff); err != nil {
		return false, errors.Wrapf(err, "failed to unmarshal patch data into a map")
	}
	return len(patchDiff) > 0, nil
}
