package names

import "fmt"

const (
	maxNameLength          = 63
	randomLength           = 5
	maxGeneratedNameLength = maxNameLength - randomLength
)

// BuildName builds a valid name from the base name, adding a suffix to the
// the base. If base is valid, the returned name must also be valid.
func BuildName(base, suffix string) string {
	if len(base) > maxGeneratedNameLength {
		base = base[:maxGeneratedNameLength]
	}
	return fmt.Sprintf("%s%s", base, suffix)
}
