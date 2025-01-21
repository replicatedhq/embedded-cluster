package systemd

import (
	"context"
	"fmt"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sirupsen/logrus"
)

var (
	_ dbusInterface = (*DBus)(nil)
)

type dbusInterface interface {
	EnableAndStart(ctx context.Context, unit string) error
	Stop(ctx context.Context, unit string) error
	Disable(ctx context.Context, unit string) error
	Restart(ctx context.Context, unit string) error
	IsActive(ctx context.Context, unit string) (bool, error)
	IsEnabled(ctx context.Context, unit string) (bool, error)
	UnitExists(ctx context.Context, unit string) (bool, error)
	Reload(ctx context.Context) error
}

// DBus is a systemd helper that uses the DBus API to run systemctl equivalent commands.
type DBus struct{}

// EnableAndStart instructs systemd to start a unit and enables its unit files.
func (d *DBus) EnableAndStart(ctx context.Context, unit string) error {
	logrus.Debugf("Enabling and starting systemd unit %q", unit)

	conn, err := newDBusConn(ctx)
	if err != nil {
		return fmt.Errorf("new dbus connection: %w", err)
	}
	defer conn.Close()

	unit = normalizeUnitName(unit)

	_, _, err = conn.EnableUnitFilesContext(ctx, []string{unit}, false, false)
	if err != nil {
		return fmt.Errorf("enable unit: %w", err)
	}

	logrus.Debugf("Successfully enabled systemd unit %q", unit)

	ch := make(chan string)
	_, err = conn.StartUnitContext(ctx, unit, "replace", ch)
	if err != nil {
		return fmt.Errorf("start unit: %w", err)
	}

	result := <-ch
	logrus.Debugf("Start systemd unit %q got result %q", unit, result)

	switch result {
	case "done":
		logrus.Debugf("Successfully started systemd unit %q", unit)
		return nil

	default:
		return fmt.Errorf("failed to start systemd unit, %q expected %q but received %q", unit, "done", result)
	}
}

// Restart restarts a systemd service. If a service is restarted that isn't running it will be
// started.
func (d *DBus) Restart(ctx context.Context, unit string) error {
	logrus.Debugf("Restarting systemd unit %q", unit)

	conn, err := newDBusConn(ctx)
	if err != nil {
		return fmt.Errorf("new dbus connection: %w", err)
	}
	defer conn.Close()

	unit = normalizeUnitName(unit)

	ch := make(chan string)
	_, err = conn.RestartUnitContext(ctx, unit, "replace", ch)
	if err != nil {
		return fmt.Errorf("restart unit: %w", err)
	}

	result := <-ch
	logrus.Debugf("Restart systemd unit %q got result %q", unit, result)

	switch result {
	case "done":
		logrus.Debugf("Successfully restarted systemd unit %q", unit)
		return nil

	default:
		return fmt.Errorf("failed to restart systemd unit, %q expected %q but received %q", unit, "done", result)
	}
}

// Stop stops a systemd service.
func (d *DBus) Stop(ctx context.Context, unit string) error {
	logrus.Debugf("Stopping systemd unit %q", unit)

	conn, err := newDBusConn(ctx)
	if err != nil {
		return fmt.Errorf("new dbus connection: %w", err)
	}
	defer conn.Close()

	unit = normalizeUnitName(unit)

	isActive, err := d.IsActive(ctx, unit)
	if err != nil {
		return fmt.Errorf("check if active: %w", err)
	}
	if !isActive {
		return nil
	}

	ch := make(chan string)
	_, err = conn.StopUnitContext(ctx, unit, "replace", ch)
	if err != nil {
		return fmt.Errorf("stop unit: %w", err)
	}

	result := <-ch
	logrus.Debugf("Stop systemd unit %q got result %q", unit, result)

	switch result {
	case "done":
		logrus.Debugf("Successfully stopped systemd unit %q", unit)
		return nil

	default:
		return fmt.Errorf("failed to stop systemd unit, %q expected %q but received %q", unit, "done", result)
	}
}

// Disable disables a systemd service.
func (d *DBus) Disable(ctx context.Context, unit string) error {
	logrus.Debugf("Disabling systemd unit %q", unit)

	conn, err := newDBusConn(ctx)
	if err != nil {
		return fmt.Errorf("new dbus connection: %w", err)
	}
	defer conn.Close()

	unit = normalizeUnitName(unit)

	isEnabled, err := d.IsEnabled(ctx, unit)
	if err != nil {
		return fmt.Errorf("check if enabled: %w", err)
	}
	if !isEnabled {
		return nil
	}

	_, err = conn.DisableUnitFilesContext(ctx, []string{unit}, false)
	if err != nil {
		return fmt.Errorf("disable unit: %w", err)
	}

	logrus.Debugf("Successfully disabled systemd unit %q", unit)
	return nil
}

// IsActive checks if a systemd service is active or not.
func (d *DBus) IsActive(ctx context.Context, unit string) (bool, error) {
	conn, err := newDBusConn(ctx)
	if err != nil {
		return false, fmt.Errorf("new dbus connection: %w", err)
	}
	defer conn.Close()

	unit = normalizeUnitName(unit)

	prop, err := conn.GetUnitPropertyContext(ctx, unit, "ActiveState")
	if err != nil {
		return false, fmt.Errorf("get unit property: %w", err)
	}
	return prop.Value.String() == `"active"`, nil
}

// IsEnabled checks if a systemd service is enabled.
func (d *DBus) IsEnabled(ctx context.Context, unit string) (bool, error) {
	conn, err := newDBusConn(ctx)
	if err != nil {
		return false, fmt.Errorf("new dbus connection: %w", err)
	}
	defer conn.Close()

	unit = normalizeUnitName(unit)

	prop, err := conn.GetUnitPropertyContext(ctx, unit, "LoadState")
	if err != nil {
		return false, fmt.Errorf("get unit property: %w", err)
	}
	return prop.Value.String() == `"loaded"`, nil
}

func (d *DBus) UnitExists(ctx context.Context, unit string) (bool, error) {
	conn, err := newDBusConn(ctx)
	if err != nil {
		return false, fmt.Errorf("new dbus connection: %w", err)
	}
	defer conn.Close()

	unit = normalizeUnitName(unit)

	status, err := conn.ListUnitsByNamesContext(ctx, []string{unit})
	if err != nil {
		return false, fmt.Errorf("list units: %w", err)
	}

	return len(status) > 0, nil
}

// Reload instructs systemd to reload the unit files.
func (d *DBus) Reload(ctx context.Context) error {
	conn, err := newDBusConn(ctx)
	if err != nil {
		return fmt.Errorf("new dbus connection: %w", err)
	}
	defer conn.Close()

	return conn.ReloadContext(ctx)
}

func newDBusConn(ctx context.Context) (*dbus.Conn, error) {
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
