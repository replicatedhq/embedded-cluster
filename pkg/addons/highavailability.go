package addons

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CanEnableHA checks if high availability can be enabled in the cluster.
func CanEnableHA(ctx context.Context, kcli client.Client) (bool, error) {
	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return false, errors.Wrap(err, "get latest installation")
	}
	if in.Spec.HighAvailability {
		return false, nil
	}

	if err := kcli.Get(ctx, types.NamespacedName{Name: constants.EcRestoreStateCMName, Namespace: "embedded-cluster"}, &corev1.ConfigMap{}); err == nil {
		return false, nil // cannot enable HA during a restore
	} else if !k8serrors.IsNotFound(err) {
		return false, errors.Wrap(err, "get restore state configmap")
	}

	ncps, err := kubeutils.NumOfControlPlaneNodes(ctx, kcli)
	if err != nil {
		return false, errors.Wrap(err, "check control plane nodes")
	}
	return ncps >= 3, nil
}

// EnableHA enables high availability.
func EnableHA(ctx context.Context, kcli client.Client, hcli helm.Client, isAirgap bool, serviceCIDR string, proxy *ecv1beta1.ProxySpec, cfgspec *ecv1beta1.ConfigSpec) error {
	loading := spinner.Start()
	defer loading.Close()

	logrus.Debugf("Enabling high availability")

	if isAirgap {
		loading.Infof("Enabling high availability")

		domains := runtimeconfig.GetDomains(cfgspec)

		// TODO (@salah): add support for end user overrides
		sw := &seaweedfs.SeaweedFS{
			ServiceCIDR:         serviceCIDR,
			ProxyRegistryDomain: domains.ProxyRegistryDomain,
		}
		exists, err := hcli.ReleaseExists(ctx, sw.Namespace(), sw.ReleaseName())
		if err != nil {
			return errors.Wrap(err, "check if seaweedfs release exists")
		}
		if !exists {
			logrus.Debugf("Installing seaweedfs")
			if err := sw.Install(ctx, kcli, hcli, addOnOverrides(sw, cfgspec, nil), nil); err != nil {
				return errors.Wrap(err, "install seaweedfs")
			}
			logrus.Debugf("Seaweedfs installed!")
		} else {
			logrus.Debugf("Seaweedfs already installed")
		}

		// TODO (@salah): add support for end user overrides
		reg := &registry.Registry{
			ServiceCIDR:         serviceCIDR,
			ProxyRegistryDomain: domains.ProxyRegistryDomain,
			IsHA:                true,
		}
		logrus.Debugf("Migrating registry data")
		if err := reg.Migrate(ctx, kcli, loading); err != nil {
			return errors.Wrap(err, "migrate registry data")
		}
		logrus.Debugf("Registry migration complete!")
		logrus.Debugf("Upgrading registry")
		if err := reg.Upgrade(ctx, kcli, hcli, addOnOverrides(reg, cfgspec, nil)); err != nil {
			return errors.Wrap(err, "upgrade registry")
		}
		logrus.Debugf("Registry upgraded!")
	}

	loading.Infof("Updating the Admin Console for high availability")

	logrus.Debugf("Enabling admin console high availability")
	err := EnableAdminConsoleHA(ctx, kcli, hcli, isAirgap, serviceCIDR, proxy, cfgspec)
	if err != nil {
		return errors.Wrap(err, "enable admin console high availability")
	}
	logrus.Debugf("Admin console high availability enabled!")

	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "get latest installation")
	}

	if err := kubeutils.UpdateInstallation(ctx, kcli, in, func(in *ecv1beta1.Installation) {
		in.Spec.HighAvailability = true
	}); err != nil {
		return errors.Wrap(err, "update installation")
	}

	logrus.Debugf("High availability enabled!")
	loading.Infof("High availability enabled!")
	return nil
}

// EnableAdminConsoleHA enables high availability for the admin console.
func EnableAdminConsoleHA(ctx context.Context, kcli client.Client, hcli helm.Client, isAirgap bool, serviceCIDR string, proxy *ecv1beta1.ProxySpec, cfgspec *ecv1beta1.ConfigSpec) error {
	domains := runtimeconfig.GetDomains(cfgspec)

	// TODO (@salah): add support for end user overrides
	ac := &adminconsole.AdminConsole{
		IsAirgap:                 isAirgap,
		IsHA:                     true,
		Proxy:                    proxy,
		ServiceCIDR:              serviceCIDR,
		ReplicatedAppDomain:      domains.ReplicatedAppDomain,
		ProxyRegistryDomain:      domains.ProxyRegistryDomain,
		ReplicatedRegistryDomain: domains.ReplicatedRegistryDomain,
	}
	if err := ac.Upgrade(ctx, kcli, hcli, addOnOverrides(ac, cfgspec, nil)); err != nil {
		return errors.Wrap(err, "upgrade admin console")
	}

	return nil
}
