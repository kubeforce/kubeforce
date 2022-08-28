package ansible

import "context"

var _ PackageManager = &yumPkgManager{}

type yumPkgManager struct {
}

func (a yumPkgManager) Update(ctx context.Context) error {
	return runCmd(ctx, "sudo", "yum", "-y", "update")
}

func (a yumPkgManager) Install(ctx context.Context, packages ...string) error {
	args := append([]string{"sudo", "yum", "-y", "install"}, packages...)
	return runCmd(ctx, args...)
}
