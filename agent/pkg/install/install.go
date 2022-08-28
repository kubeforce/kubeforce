package install

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/pkg/errors"
)

var (
	//go:embed assets/kubeforce-agent.service
	agentServiceContent string
)

// Install installs a agent as the systemd service and runs it
func Install(ctx context.Context, cfgFile string) error {
	if err := copyBinary(); err != nil {
		return err
	}
	if err := copyConfig(cfgFile); err != nil {
		return err
	}
	if err := createService(ctx); err != nil {
		return err
	}
	return nil
}

func copyBinary() error {
	exPath, err := os.Executable()
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Dir(agentPath), 0755); err != nil {
		return err
	}
	if _, err := copy(exPath, agentPath, 0755); err != nil {
		return err
	}
	if err := os.Chmod(agentPath, 0755); err != nil {
		return err
	}
	if err := os.Chown(agentPath, 0, 0); err != nil {
		return err
	}
	return nil
}

func copyConfig(cfgFile string) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0600); err != nil {
		return err
	}
	if _, err := copy(cfgFile, configPath, 0600); err != nil {
		return err
	}
	if err := os.Chmod(configPath, 0600); err != nil {
		return err
	}
	return nil
}

func createService(ctx context.Context) error {
	if err := ioutil.WriteFile(servicePath, []byte(agentServiceContent), 0644); err != nil {
		return err
	}
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}

	if err := conn.ReloadContext(ctx); err != nil {
		return errors.Wrapf(err, "unable to reload unit files")
	}
	_, _, err = conn.EnableUnitFilesContext(ctx, []string{serviceName}, false, false)
	if err != nil {
		return err
	}

	responseCh := make(chan string)
	if _, err := conn.StartUnitContext(ctx, serviceName, "replace", responseCh); err != nil {
		return errors.Wrapf(err, "unable to start unit %s", serviceName)
	}

	response := <-responseCh
	fmt.Printf("service %q has been started. response: %s\n", serviceName, response)
	return nil
}

func copy(src, dst string, dstMode os.FileMode) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, dstMode)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func systemdUnitRunning(name string) (bool, error) {
	ctx := context.Background()
	conn, err := dbus.NewWithContext(ctx)
	defer conn.Close()
	if err != nil {
		return false, err
	}
	units, err := conn.ListUnitsContext(ctx)
	if err != nil {
		return false, err
	}
	for _, unit := range units {
		if unit.Name == name+".service" {
			return unit.ActiveState == "active", nil
		}
	}
	return true, nil
}
