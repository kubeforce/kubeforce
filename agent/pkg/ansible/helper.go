package ansible

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/atomic"
)

const (
	ansibleCmd = "ansible"
)

// GetHelper returns an ansible helper
func GetHelper() Helper {
	return singleton
}

var singleton Helper = &helper{
	ansibleInstalled: atomic.NewBool(false),
	mu:               sync.Mutex{},
}

// Helper describes interface for ansible helper
type Helper interface {
	// EnsureAnsible installs Ansible if it is not on the host.
	EnsureAnsible(ctx context.Context) error
}

type helper struct {
	ansibleInstalled *atomic.Bool
	mu               sync.Mutex
}

func (h *helper) EnsureAnsible(ctx context.Context) error {
	if h.ansibleInstalled.Load() {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if isCommandAvailable(ctx, ansibleCmd) {
		err := runCmd(ctx, ansibleCmd, "--version")
		if err != nil {
			return errors.WithStack(err)
		}
		h.ansibleInstalled.Store(true)
		return nil
	}
	if err := h.installAnsible(ctx); err != nil {
		return errors.Wrap(err, "unable to install ansible")
	}
	h.ansibleInstalled.Store(true)
	return nil
}

func (h *helper) installAnsible(ctx context.Context) error {
	pkgManager, err := GetPackageManager(ctx)
	if err != nil {
		return err
	}
	if err := pkgManager.Update(ctx); err != nil {
		return err
	}
	if isCommandAvailable(ctx, "pip") {
		if err := pkgManager.Install(ctx, "python-pip"); err != nil {
			return err
		}
	}
	if err := runCmd(ctx, "pip", "--version"); err != nil {
		return err
	}
	if err := runCmd(ctx, "pip", "install", "ansible"); err != nil {
		return err
	}
	return runCmd(ctx, ansibleCmd, "--version")
}
