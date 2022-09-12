package patch

import (
	"encoding/json"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HasChanges checks the patch data to determine if this object has changes or not
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
