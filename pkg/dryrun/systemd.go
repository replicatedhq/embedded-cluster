package dryrun

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
)

var _ systemd.Interface = (*Systemd)(nil)

type Systemd struct{}

func (s *Systemd) EnableAndStart(ctx context.Context, unit string) error {
	RecordCommand("systemctl", []string{"enable", "--now", unit}, nil)
	return nil
}

func (s *Systemd) Stop(ctx context.Context, unit string) error {
	RecordCommand("systemctl", []string{"stop", unit}, nil)
	return nil
}

func (s *Systemd) Disable(ctx context.Context, unit string) error {
	RecordCommand("systemctl", []string{"disable", unit}, nil)
	return nil
}

func (s *Systemd) Restart(ctx context.Context, unit string) error {
	RecordCommand("systemctl", []string{"restart", unit}, nil)
	return nil
}

func (s *Systemd) IsActive(ctx context.Context, unit string) (bool, error) {
	RecordCommand("systemctl", []string{"is-active", unit}, nil)
	return false, nil
}

func (s *Systemd) IsEnabled(ctx context.Context, unit string) (bool, error) {
	RecordCommand("systemctl", []string{"is-enabled", unit}, nil)
	return false, nil
}

func (s *Systemd) UnitExists(ctx context.Context, unit string) (bool, error) {
	RecordCommand("systemctl", []string{"list-units", unit}, nil)
	return false, nil
}

func (s *Systemd) Reload(ctx context.Context) error {
	RecordCommand("systemctl", []string{"daemon-reload"}, nil)
	return nil
}
