package ansible

import (
	"context"

	"github.com/pkg/errors"
)

type PackageManager interface {
	Update(ctx context.Context) error
	Install(ctx context.Context, packages ...string) error
}

func GetPackageManager(ctx context.Context) (PackageManager, error) {
	if isCommandAvailable(ctx, "apt-get") {
		return &aptPkgManager{}, nil
	} else if isCommandAvailable(ctx, "yum") {
		return &yumPkgManager{}, nil
	}
	return nil, errors.New("this system uses an unknown package manager")
}
