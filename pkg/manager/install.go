package manager

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

var (
	managerDropInFileContents = `[Service]
# Empty ExecStart= will clear out the previous ExecStart value
ExecStart=
ExecStart=%s start
`
)

type LogFunc func(string, ...interface{})

// UnitName returns the name of the systemd unit for the manager service.
func UnitName() string {
	return fmt.Sprintf("%s.service", runtimeconfig.ManagerServiceName)
}

// Install installs and starts the manager service.
func Install(ctx context.Context, logf LogFunc, m *goods.Materializer) error {
	logf("Writing manager systemd unit file")
	err := writeSystemdUnitFile(m)
	if err != nil {
		return fmt.Errorf("write manager systemd unit file: %w", err)
	}
	logf("Successfully wrote manager systemd unit file")

	logrus.Infof("Writing manager drop-in file")
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

	_, err = os.Stat(DropInDirPath())
	if err == nil {
		logf("Removing manager drop-in directory")
		if err := helpers.RemoveAll(DropInDirPath()); err != nil {
			return fmt.Errorf("remove manager drop-in directory: %w", err)
		}
		logf("Successfully removed manager drop-in directory")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check if drop-in directory exists: %w", err)
	}

	_, err = os.Stat(SystemdUnitFilePath())
	if err == nil {
		logf("Removing manager systemd unit file")
		if err := helpers.RemoveAll(SystemdUnitFilePath()); err != nil {
			return fmt.Errorf("remove manager systemd unit file: %w", err)
		}
		logf("Successfully removed manager systemd unit file")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check if systemd unit file exists: %w", err)
	}

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

func systemdUnitFileContents(m *goods.Materializer) ([]byte, error) {
	return m.ManagerUnitFileContents()
}

// writeSystemdUnitFile writes the manager systemd unit file.
func writeSystemdUnitFile(m *goods.Materializer) error {
	contents, err := systemdUnitFileContents(m)
	if err != nil {
		return fmt.Errorf("read unit file: %w", err)
	}
	err = systemd.WriteUnitFile(UnitName(), contents)
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
