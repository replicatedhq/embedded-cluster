package firewalld

import (
	"context"
	"errors"
	"os/exec"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
)

const (
	firewallCmd         = "firewall-cmd"
	exitCodeInvalidZone = 112
)

// Client is a client for the firewalld API.
type Client struct {
}

type Option func(c *Context)

type Context struct {
	Permanent bool
	Zone      string
}

func (c *Context) Args() []string {
	var args []string
	if c.Permanent {
		args = append(args, "--permanent")
	}
	if c.Zone != "" {
		args = append(args, "--zone", c.Zone)
	}
	return args
}

func IsPermanent() Option {
	return func(c *Context) {
		c.Permanent = true
	}
}

func WithZone(zone string) Option {
	return func(c *Context) {
		c.Zone = zone
	}
}

// ZoneExists checks if a zone exists.
func (c *Client) ZoneExists(ctx context.Context, zone string) (bool, error) {
	opts := commandOptions(ctx)
	err := helpers.RunCommandWithOptions(opts, firewallCmd, "--info-zone", zone)
	if err != nil {
		exitErr := new(exec.ExitError)
		if errors.As(err, &exitErr) && exitErr.ExitCode() == exitCodeInvalidZone {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// NewZone creates a new zone.
func (c *Client) NewZone(ctx context.Context, zone string, opts ...Option) error {
	args := []string{"--new-zone", zone}
	args = append(args, buildContext(opts...).Args()...)
	return helpers.RunCommandWithOptions(commandOptions(ctx), firewallCmd, args...)
}

// SetZoneTarget sets the target for a zone.
func (c *Client) SetZoneTarget(ctx context.Context, target string, opts ...Option) error {
	args := []string{"--set-target", target}
	args = append(args, buildContext(opts...).Args()...)
	return helpers.RunCommandWithOptions(commandOptions(ctx), firewallCmd, args...)
}

// AddSourceToZone adds a source to a  zone.
func (c *Client) AddSourceToZone(ctx context.Context, source string, opts ...Option) error {
	args := []string{"--add-source", source}
	args = append(args, buildContext(opts...).Args()...)
	return helpers.RunCommandWithOptions(commandOptions(ctx), firewallCmd, args...)
}

// AddInterfaceToZone adds an interface to a zone.
func (c *Client) AddInterfaceToZone(ctx context.Context, iface string, opts ...Option) error {
	args := []string{"--add-interface", iface}
	args = append(args, buildContext(opts...).Args()...)
	return helpers.RunCommandWithOptions(commandOptions(ctx), firewallCmd, args...)
}

// AddPortToZone adds a port to a zone.
func (c *Client) AddPortToZone(ctx context.Context, port string, opts ...Option) error {
	args := []string{"--add-port", port}
	args = append(args, buildContext(opts...).Args()...)
	return helpers.RunCommandWithOptions(commandOptions(ctx), firewallCmd, args...)
}

// Reload reloads the firewalld configuration.
func (c *Client) Reload(ctx context.Context) error {
	opts := commandOptions(ctx)
	return helpers.RunCommandWithOptions(opts, firewallCmd, "--reload")
}

func buildContext(opts ...Option) *Context {
	c := &Context{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func commandOptions(ctx context.Context) helpers.RunCommandOptions {
	return helpers.RunCommandOptions{
		Context: ctx,
	}
}
