package hostutils

import (
	"context"
)

var h HostUtilsInterface

func init() {
	Set(New())
}

func Set(_h HostUtilsInterface) {
	h = _h
}

type HostUtilsInterface interface {
	ConfigureForInstall(ctx context.Context, opts InitForInstallOptions) error
	ConfigureSysctl() error
	ConfigureKernelModules() error
	ConfigureNetworkManager(ctx context.Context) error
	ConfigureFirewalld(ctx context.Context, podNetwork, serviceNetwork string) error
	ResetFirewalld(ctx context.Context) error
	MaterializeFiles(airgapBundle string) error
}

// Convenience functions
// TODO (@salah): can be removed once CLI uses API for host operations)

func ConfigureForInstall(ctx context.Context, opts InitForInstallOptions) error {
	return h.ConfigureForInstall(ctx, opts)
}

func ConfigureSysctl() error {
	return h.ConfigureSysctl()
}

func ConfigureKernelModules() error {
	return h.ConfigureKernelModules()
}

func ConfigureNetworkManager(ctx context.Context) error {
	return h.ConfigureNetworkManager(ctx)
}

func ConfigureFirewalld(ctx context.Context, podNetwork, serviceNetwork string) error {
	return h.ConfigureFirewalld(ctx, podNetwork, serviceNetwork)
}

func ResetFirewalld(ctx context.Context) error {
	return h.ResetFirewalld(ctx)
}

func MaterializeFiles(airgapBundle string) error {
	return h.MaterializeFiles(airgapBundle)
}
