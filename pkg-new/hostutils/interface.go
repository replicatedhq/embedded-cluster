package hostutils

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

var h HostUtilsInterface

func init() {
	Set(New())
}

func Set(_h HostUtilsInterface) {
	h = _h
}

type HostUtilsInterface interface {
	ConfigureHost(ctx context.Context, rc runtimeconfig.RuntimeConfig, channelRelease *release.ChannelRelease, opts InitForInstallOptions) error
	ConfigureSysctl() error
	ConfigureKernelModules() error
	ConfigureNetworkManager(ctx context.Context, rc runtimeconfig.RuntimeConfig) error
	ConfigureFirewalld(ctx context.Context, podNetwork, serviceNetwork string) error
	ResetFirewalld(ctx context.Context) error
	MaterializeFiles(rc runtimeconfig.RuntimeConfig, channelRelease *release.ChannelRelease, airgapBundle string) error
	CreateSystemdUnitFiles(ctx context.Context, logger logrus.FieldLogger, rc runtimeconfig.RuntimeConfig, hostname string, isWorker bool) error
	WriteLocalArtifactMirrorDropInFile(rc runtimeconfig.RuntimeConfig) error
	AddInsecureRegistry(registry string) error
	ConfigureSELinuxFcontext(rc runtimeconfig.RuntimeConfig) error
	RestoreSELinuxContext(rc runtimeconfig.RuntimeConfig) error
}

// Convenience functions
// TODO (@salah): can be removed once CLI uses API for host operations)

func ConfigureHost(ctx context.Context, rc runtimeconfig.RuntimeConfig, channelRelease *release.ChannelRelease, opts InitForInstallOptions) error {
	return h.ConfigureHost(ctx, rc, channelRelease, opts)
}

func ConfigureSysctl() error {
	return h.ConfigureSysctl()
}

func ConfigureKernelModules() error {
	return h.ConfigureKernelModules()
}

func ConfigureNetworkManager(ctx context.Context, rc runtimeconfig.RuntimeConfig) error {
	return h.ConfigureNetworkManager(ctx, rc)
}

func ConfigureFirewalld(ctx context.Context, podNetwork, serviceNetwork string) error {
	return h.ConfigureFirewalld(ctx, podNetwork, serviceNetwork)
}

func ResetFirewalld(ctx context.Context) error {
	return h.ResetFirewalld(ctx)
}

func MaterializeFiles(rc runtimeconfig.RuntimeConfig, channelRelease *release.ChannelRelease, airgapBundle string) error {
	return h.MaterializeFiles(rc, channelRelease, airgapBundle)
}

func CreateSystemdUnitFiles(ctx context.Context, logger logrus.FieldLogger, rc runtimeconfig.RuntimeConfig, hostname string, isWorker bool) error {
	return h.CreateSystemdUnitFiles(ctx, logger, rc, hostname, isWorker)
}

func WriteLocalArtifactMirrorDropInFile(rc runtimeconfig.RuntimeConfig) error {
	return h.WriteLocalArtifactMirrorDropInFile(rc)
}

func AddInsecureRegistry(registry string) error {
	return h.AddInsecureRegistry(registry)
}

func ConfigureSELinuxFcontext(rc runtimeconfig.RuntimeConfig) error {
	return h.ConfigureSELinuxFcontext(rc)
}

func RestoreSELinuxContext(rc runtimeconfig.RuntimeConfig) error {
	return h.RestoreSELinuxContext(rc)
}
