package manager

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

var (
	//go:embed static/manager.service
	_systemdUnitFileContents []byte

	managerDropInFileContents = `[Service]
# Empty ExecStart= will clear out the previous ExecStart value
ExecStart=
ExecStart=%s start
`
)

type LogFunc func(string, ...interface{})

const (
	DefaultServiceName = "manager"
)

var (
	_serviceName = DefaultServiceName
)

// ServiceName returns the name of the systemd service for the manager service.
func ServiceName() string {
	return _serviceName
}

// SetServiceName sets the name of the systemd service for the manager service.
func SetServiceName(appSlug string) {
	_serviceName = fmt.Sprintf("%s-manager", appSlug)
}

// UnitName returns the name of the systemd unit for the manager service.
func UnitName() string {
	return fmt.Sprintf("%s.service", ServiceName())
}

// Install installs and starts the manager service.
func Install(ctx context.Context, logf LogFunc) error {
	logf("Writing manager systemd unit file")
	err := writeSystemdUnitFile()
	if err != nil {
		return fmt.Errorf("write manager systemd unit file: %w", err)
	}
	logf("Successfully wrote manager systemd unit file")

	logf("Writing manager drop-in file")
	if err := writeDropInFile(); err != nil {
		return fmt.Errorf("write manager drop-in file: %w", err)
	}
	logf("Successfully wrote manager drop-in file")

	logf("Enabling and starting manager service")
	if err := systemd.EnableAndStart(ctx, UnitName()); err != nil {
		return fmt.Errorf("enable and start manager service: %w", err)
	}
	logf("Successfully enabled and started manager service")

	return nil
}

// Uninstall stops and disables the manager service.
func Uninstall(ctx context.Context, logf LogFunc) error {
	exists, err := systemd.UnitExists(ctx, UnitName())
	if err != nil {
		return fmt.Errorf("check if unit exists: %w", err)
	}
	if !exists {
		logf("Manager service does not exist, nothing to uninstall")
	} else {
		logf("Stopping manager service")
		err := systemd.Stop(ctx, UnitName())
		if err != nil {
			return fmt.Errorf("systemd stop: %w", err)
		}
		logf("Successfully stopped manager service")

		logf("Disabling manager service")
		err = systemd.Disable(ctx, UnitName())
		if err != nil {
			return fmt.Errorf("systemd disable: %w", err)
		}
		logf("Successfully disabled manager service")
	}

	logf("Removing manager drop-in directory")
	err = os.RemoveAll(DropInDirPath())
	if err != nil {
		return fmt.Errorf("remove manager drop-in directory: %w", err)
	}
	logf("Successfully removed manager drop-in directory")

	logf("Removing manager systemd unit file")
	err = os.RemoveAll(SystemdUnitFilePath())
	if err != nil {
		return fmt.Errorf("remove manager systemd unit file: %w", err)
	}
	logf("Successfully removed manager systemd unit file")

	return nil
}

// SystemdUnitFilePath returns the path to the systemd unit file for the manager service.
func SystemdUnitFilePath() string {
	return systemd.UnitFilePath(UnitName())
}

// DropInDirPath returns the path to the manager drop-in directory.
func DropInDirPath() string {
	return systemd.DropInDirPath(UnitName())
}

// writeSystemdUnitFile writes the manager systemd unit file.
func writeSystemdUnitFile() error {
	err := systemd.WriteUnitFile(UnitName(), _systemdUnitFileContents)
	if err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	return nil
}

// writeDropInFile writes the manager drop-in file.
func writeDropInFile() error {
	contents := fmt.Sprintf(
		managerDropInFileContents,
		runtimeconfig.PathToEmbeddedClusterBinary("manager"),
	)
	err := systemd.WriteDropInFile(UnitName(), "embedded-cluster.conf", []byte(contents))
	if err != nil {
		return fmt.Errorf("write drop-in file: %w", err)
	}
	err = systemd.Reload(context.Background())
	if err != nil {
		return fmt.Errorf("systemd reload: %w", err)
	}
	return nil
}
