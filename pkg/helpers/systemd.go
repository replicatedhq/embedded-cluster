package helpers

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
)

// IsSystemdServiceActive checks if a systemd service is active or not.
func IsSystemdServiceActive(ctx context.Context, svcname string) (bool, error) {
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return false, fmt.Errorf("unable to establish connection to systemd: %w", err)
	}
	defer conn.Close()
	if !strings.HasSuffix(svcname, ".service") {
		svcname = fmt.Sprintf("%s.service", svcname)
	}
	prop, err := conn.GetUnitPropertyContext(ctx, svcname, "ActiveState")
	if err != nil {
		return false, fmt.Errorf("unable to get service property: %w", err)
	}
	return prop.Value.String() == `"active"`, nil
}
