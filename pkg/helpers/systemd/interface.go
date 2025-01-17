package systemd

import "context"

var _h Interface

var _ Interface = (*Systemd)(nil)

func init() {
	Set(&Systemd{})
}

func Set(h Interface) {
	_h = h
}

// Interface is an interface that wraps the helper commands.
type Interface interface {
	dbusInterface
}

type Systemd struct {
	DBus
}

func EnableAndStart(ctx context.Context, unit string) error {
	return _h.EnableAndStart(ctx, unit)
}

func Stop(ctx context.Context, unit string) error {
	return _h.Stop(ctx, unit)
}

func Disable(ctx context.Context, unit string) error {
	return _h.Disable(ctx, unit)
}

func Restart(ctx context.Context, unit string) error {
	return _h.Restart(ctx, unit)
}

func IsActive(ctx context.Context, unit string) (bool, error) {
	return _h.IsActive(ctx, unit)
}

func IsEnabled(ctx context.Context, unit string) (bool, error) {
	return _h.IsEnabled(ctx, unit)
}

func UnitExists(ctx context.Context, unit string) (bool, error) {
	return _h.UnitExists(ctx, unit)
}

func Reload(ctx context.Context) error {
	return _h.Reload(ctx)
}
