package ansible

import (
	"context"
	"os/exec"

	"github.com/pkg/errors"
)

func runCmd(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "unable to execute cmd: %q", cmd)
	}
	return nil
}

func isCommandAvailable(ctx context.Context, name string) bool {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "command -v "+name)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
