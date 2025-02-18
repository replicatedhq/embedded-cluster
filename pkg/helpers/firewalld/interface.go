package firewalld

import "context"

var _f Interface

var _ Interface = (*Client)(nil)

func init() {
	Set(&Client{})
}

// Set sets the firewalld client.
func Set(f Interface) {
	_f = f
}

// Interface is an interface that wraps the firewalld commands.
type Interface interface {
	// ZoneExists checks if a zone exists.
	ZoneExists(ctx context.Context, zone string) (bool, error)
	// NewZone creates a new zone.
	NewZone(ctx context.Context, zone string, opts ...Option) error
	// SetZoneTarget sets the target for a zone.
	SetZoneTarget(ctx context.Context, target string, opts ...Option) error
	// AddSourceToZone adds a source to a zone.
	AddSourceToZone(ctx context.Context, source string, opts ...Option) error
	// AddInterfaceToZone adds an interface to a zone.
	AddInterfaceToZone(ctx context.Context, iface string, opts ...Option) error
	// AddPortToZone adds a port to a zone.
	AddPortToZone(ctx context.Context, port string, opts ...Option) error
	// Reload reloads the firewalld configuration.
	Reload(ctx context.Context) error
}

// ZoneExists checks if a zone exists.
func ZoneExists(ctx context.Context, zone string) (bool, error) {
	return _f.ZoneExists(ctx, zone)
}

// NewZone creates a new  zone.
func NewZone(ctx context.Context, zone string, opts ...Option) error {
	return _f.NewZone(ctx, zone, opts...)
}

// SetZoneTarget sets the target for a zone.
func SetZoneTarget(ctx context.Context, target string, opts ...Option) error {
	return _f.SetZoneTarget(ctx, target, opts...)
}

// AddSourceToZone adds a source to a zone.
func AddSourceToZone(ctx context.Context, source string, opts ...Option) error {
	return _f.AddSourceToZone(ctx, source, opts...)
}

// AddInterfaceToZone adds an interface to a zone.
func AddInterfaceToZone(ctx context.Context, iface string, opts ...Option) error {
	return _f.AddInterfaceToZone(ctx, iface, opts...)
}

// AddPortToZone adds a port to a zone.
func AddPortToZone(ctx context.Context, port string, opts ...Option) error {
	return _f.AddPortToZone(ctx, port, opts...)
}

// Reload reloads the firewalld configuration.
func Reload(ctx context.Context) error {
	return _f.Reload(ctx)
}
