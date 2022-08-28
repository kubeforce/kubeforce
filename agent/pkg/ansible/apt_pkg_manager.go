package ansible

import "context"

var _ PackageManager = &aptPkgManager{}

type aptPkgManager struct {
}

func (a aptPkgManager) Update(ctx context.Context) error {
	return runCmd(ctx, "sudo", "apt-get", "update")
}

func (a aptPkgManager) Install(ctx context.Context, packages ...string) error {
	args := append([]string{"sudo", "apt-get", "install", "-y"}, packages...)
	return runCmd(ctx, args...)
}
