package manager

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

var (
	managerDropInFileContents = `[Service]
# Empty ExecStart= will clear out the previous ExecStart value
ExecStart=
ExecStart=%s start
`
)

// UnitName returns the name of the systemd unit for the manager service.
func UnitName() string {
	return fmt.Sprintf("%s.service", runtimeconfig.ManagerServiceName)
}

// SystemdUnitFilePath returns the path to the systemd unit file for the manager service.
func SystemdUnitFilePath() string {
	return systemd.UnitFilePath(UnitName())
}

// SystemdUnitFileContents returns the contents of the systemd unit file for the manager service.
func SystemdUnitFileContents(m *goods.Materializer) ([]byte, error) {
	return m.ManagerUnitFileContents()
}

// WriteSystemdUnitFile writes the manager systemd unit file.
func WriteSystemdUnitFile(m *goods.Materializer) error {
	contents, err := SystemdUnitFileContents(m)
	if err != nil {
		return fmt.Errorf("read unit file: %w", err)
	}
	err = systemd.WriteUnitFile(UnitName(), contents)
	if err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	return nil
}

// WriteDropInFile writes the manager drop-in file.
func WriteDropInFile() error {
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
		return fmt.Errorf("reload systemd: %w", err)
	}
	return nil
}

// EnableAndStart enables and starts the manager service.
func EnableAndStart(ctx context.Context) error {
	return systemd.EnableAndStart(ctx, UnitName())
}

// Restart restarts the manager service.
func Restart(ctx context.Context) error {
	return systemd.Restart(ctx, UnitName())
}
