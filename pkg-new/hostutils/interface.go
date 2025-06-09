package hostutils

import (
	"context"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
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
	ConfigureForInstall(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts InitForInstallOptions) error
	ConfigureSysctl() error
	ConfigureKernelModules() error
	ConfigureNetworkManager(ctx context.Context, rc runtimeconfig.RuntimeConfig) error
	ConfigureFirewalld(ctx context.Context, podNetwork, serviceNetwork string) error
	ResetFirewalld(ctx context.Context) error
	MaterializeFiles(rc runtimeconfig.RuntimeConfig, airgapBundle string) error
	CreateSystemdUnitFiles(ctx context.Context, logger logrus.FieldLogger, rc runtimeconfig.RuntimeConfig, isWorker bool, proxy *ecv1beta1.ProxySpec) error
	WriteLocalArtifactMirrorDropInFile(rc runtimeconfig.RuntimeConfig) error
}

// Convenience functions
// TODO (@salah): can be removed once CLI uses API for host operations)

func ConfigureForInstall(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts InitForInstallOptions) error {
	return h.ConfigureForInstall(ctx, rc, opts)
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

func MaterializeFiles(rc runtimeconfig.RuntimeConfig, airgapBundle string) error {
	return h.MaterializeFiles(rc, airgapBundle)
}

func CreateSystemdUnitFiles(ctx context.Context, logger logrus.FieldLogger, rc runtimeconfig.RuntimeConfig, isWorker bool, proxy *ecv1beta1.ProxySpec) error {
	return h.CreateSystemdUnitFiles(ctx, logger, rc, isWorker, proxy)
}

func WriteLocalArtifactMirrorDropInFile(rc runtimeconfig.RuntimeConfig) error {
	return h.WriteLocalArtifactMirrorDropInFile(rc)
}
