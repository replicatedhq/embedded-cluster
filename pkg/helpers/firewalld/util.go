package firewalld

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
)

var _u UtilInterface

var _ UtilInterface = (*Util)(nil)

func init() {
	SetUtil(&Util{})
}

// SetUtil sets the firewalld util.
func SetUtil(u UtilInterface) {
	_u = u
}

type UtilInterface interface {
	// IsFirewalldActive checks if firewalld is installed and active.
	IsFirewalldActive(ctx context.Context) (bool, error)
	// FirewallCmdExists checks if firewall-cmd binary exists.
	FirewallCmdExists(ctx context.Context) (bool, error)
}

// IsFirewalldActive checks if firewalld is installed and active.
func IsFirewalldActive(ctx context.Context) (bool, error) {
	return _u.IsFirewalldActive(ctx)
}

// FirewallCmdExists checks if firewall-cmd binary exists.
func FirewallCmdExists(ctx context.Context) (bool, error) {
	return _u.FirewallCmdExists(ctx)
}

type Util struct {
}

// IsFirewalldActive checks if firewalld is installed and active.
func (u *Util) IsFirewalldActive(ctx context.Context) (bool, error) {
	exists, err := systemd.UnitExists(ctx, "firewalld")
	if err != nil {
		return false, fmt.Errorf("exists: %w", err)
	} else if !exists {
		return false, nil
	}

	active, err := systemd.IsActive(ctx, "firewalld")
	if err != nil {
		return false, fmt.Errorf("active: %w", err)
	}
	return active, nil
}

// FirewallCmdExists checks if firewall-cmd binary exists.
func (u *Util) FirewallCmdExists(ctx context.Context) (bool, error) {
	_, err := exec.LookPath("firewall-cmd")
	if err != nil {
		return false, fmt.Errorf("lookpath: %w", err)
	}
	return true, nil
}
