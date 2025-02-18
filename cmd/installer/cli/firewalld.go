package cli

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers/firewalld"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
)

// configureFirewalld configures firewalld for the cluster. It adds the ec-net zone for pod and
// service communication with default target ACCEPT, and opens the necessary ports in the default
// zone for k0s and k8s components on the host network.
func configureFirewalld(ctx context.Context, podNetwork, serviceNetwork string) error {
	isActive, err := firewalld.IsFirewalldActive(ctx)
	if err != nil {
		return fmt.Errorf("check if firewalld is active: %w", err)
	}
	if !isActive {
		logrus.Debugf("firewalld is not active, skipping configuration")
		return nil
	}

	logrus.Debugf("firewalld is active, configuring")

	cmdExists, err := firewalld.FirewallCmdExists(ctx)
	if err != nil {
		return fmt.Errorf("check if firewall-cmd exists: %w", err)
	}
	if !cmdExists {
		logrus.Warn("firewall-cmd not found but firewalld is active, skipping firewalld configuration")
		return nil
	}

	err = ensureFirewalldECNetZone(ctx, podNetwork, serviceNetwork)
	if err != nil {
		return fmt.Errorf("ensure ec-net zone: %w", err)
	}

	err = ensureFirewalldDefaultZone(ctx)
	if err != nil {
		return fmt.Errorf("ensure default zone: %w", err)
	}

	err = firewalld.Reload(ctx)
	if err != nil {
		return fmt.Errorf("reload firewalld: %w", err)
	}

	return nil
}

// resetFirewalld removes all firewalld configuration added by the installer.
func resetFirewalld(ctx context.Context) (finalErr error) {
	cmdExists, err := firewalld.FirewallCmdExists(ctx)
	if err != nil {
		return fmt.Errorf("check if firewall-cmd exists: %w", err)
	}
	if !cmdExists {
		return nil
	}

	err = resetFirewalldECNetZone(ctx)
	if err != nil {
		finalErr = multierr.Append(finalErr, fmt.Errorf("reset ec-net zone: %w", err))
	}

	err = resetFirewalldDefaultZone(ctx)
	if err != nil {
		finalErr = multierr.Append(finalErr, fmt.Errorf("reset default zone: %w", err))
	}

	err = firewalld.Reload(ctx)
	if err != nil {
		return fmt.Errorf("reload firewalld: %w", err)
	}

	return
}

func ensureFirewalldECNetZone(ctx context.Context, podNetwork, serviceNetwork string) error {
	opts := []firewalld.Option{
		firewalld.IsPermanent(),
		firewalld.WithZone("ec-net"),
	}

	exists, err := firewalld.ZoneExists(ctx, "ec-net")
	if err != nil {
		return fmt.Errorf("check if ec-net zone exists: %w", err)
	} else if !exists {
		err = firewalld.NewZone(ctx, "ec-net", opts...)
		if err != nil {
			return fmt.Errorf("create ec-net zone: %w", err)
		}
	}

	// Set the default target to ACCEPT for pod and service networks
	err = firewalld.SetZoneTarget(ctx, "ACCEPT", opts...)
	if err != nil {
		return fmt.Errorf("set target to ACCEPT: %w", err)
	}

	err = firewalld.AddSourceToZone(ctx, podNetwork, opts...)
	if err != nil {
		return fmt.Errorf("add pod network source: %w", err)
	}

	err = firewalld.AddSourceToZone(ctx, serviceNetwork, opts...)
	if err != nil {
		return fmt.Errorf("add service network source: %w", err)
	}

	// Add the calico interfaces
	// This is redundant and overlaps with the pod network but we add it anyway
	calicoIfaces := []string{"cali+", "tunl+", "vxlan-v6.calico", "vxlan.calico", "wg-v6.cali", "wireguard.cali"}
	for _, iface := range calicoIfaces {
		err = firewalld.AddInterfaceToZone(ctx, iface, opts...)
		if err != nil {
			return fmt.Errorf("add %s interface: %w", iface, err)
		}
	}

	return nil
}

func resetFirewalldECNetZone(ctx context.Context) (finalErr error) {
	opts := []firewalld.Option{
		firewalld.IsPermanent(),
	}

	exists, err := firewalld.ZoneExists(ctx, "ec-net")
	if err != nil {
		return fmt.Errorf("check if ec-net zone exists: %w", err)
	} else if !exists {
		return nil
	}

	err = firewalld.DeleteZone(ctx, "ec-net", opts...)
	if err != nil {
		return fmt.Errorf("delete ec-net zone: %w", err)
	}

	return
}

func ensureFirewalldDefaultZone(ctx context.Context) error {
	opts := []firewalld.Option{
		firewalld.IsPermanent(),
	}

	// Allow other nodes to connect to k0s core components
	ports := []string{"6443/tcp", "10250/tcp", "9443/tcp", "2380/tcp", "4789/udp"}
	for _, port := range ports {
		err := firewalld.AddPortToZone(ctx, port, opts...)
		if err != nil {
			return fmt.Errorf("add %s port: %w", port, err)
		}
	}

	return nil
}

func resetFirewalldDefaultZone(ctx context.Context) (finalErr error) {
	opts := []firewalld.Option{
		firewalld.IsPermanent(),
	}

	// Allow other nodes to connect to k0s core components
	ports := []string{"6443/tcp", "10250/tcp", "9443/tcp", "2380/tcp", "4789/udp"}
	for _, port := range ports {
		err := firewalld.RemovePortFromZone(ctx, port, opts...)
		if err != nil {
			finalErr = multierr.Append(finalErr, fmt.Errorf("remove %s port: %w", port, err))
		}
	}

	return
}
