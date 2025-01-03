package systemd

import (
	"context"
	"fmt"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sirupsen/logrus"
)

// EnableAndStart instructs systemd to start a unit and enables its unit files.
func EnableAndStart(ctx context.Context, unit string) error {
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
func Restart(ctx context.Context, unit string) error {
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

// IsActive checks if a systemd service is active or not.
func IsActive(ctx context.Context, unit string) (bool, error) {
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

// Reload instructs systemd to reload the unit files.
func Reload(ctx context.Context) error {
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
