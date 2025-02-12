package addons

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	registrymigrate "github.com/replicatedhq/embedded-cluster/pkg/addons/registry/migrate"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CanEnableHA checks if high availability can be enabled in the cluster.
func CanEnableHA(ctx context.Context, kcli client.Client) (bool, string, error) {
	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return false, "", errors.Wrap(err, "get latest installation")
	}
	if in.Spec.HighAvailability {
		return false, "already enabled", nil
	}

	if err := kcli.Get(ctx, types.NamespacedName{Name: constants.EcRestoreStateCMName, Namespace: "embedded-cluster"}, &corev1.ConfigMap{}); err == nil {
		return false, "a restore is in progress", nil
	} else if !k8serrors.IsNotFound(err) {
		return false, "", errors.Wrap(err, "get restore state configmap")
	}

	ncps, err := kubeutils.NumOfControlPlaneNodes(ctx, kcli)
	if err != nil {
		return false, "", errors.Wrap(err, "check control plane nodes")
	}
	if ncps < 3 {
		return false, "number of control plane nodes is less than 3", nil
	}
	return true, "", nil
}

// EnableHA enables high availability.
func EnableHA(ctx context.Context, kcli client.Client, kclient kubernetes.Interface, hcli helm.Client, isAirgap bool, serviceCIDR string, proxy *ecv1beta1.ProxySpec, cfgspec *ecv1beta1.ConfigSpec) error {
	loading := spinner.Start()
	defer loading.Close()

	logrus.Debugf("Enabling high availability")

	if isAirgap {
		loading.Infof("Enabling high availability")

		// TODO (@salah): add support for end user overrides
		sw := &seaweedfs.SeaweedFS{
			ServiceCIDR: serviceCIDR,
		}
		logrus.Debugf("Installing seaweedfs")
		if err := sw.Install(ctx, kcli, hcli, addOnOverrides(sw, cfgspec, nil), nil); err != nil {
			return errors.Wrap(err, "install seaweedfs")
		}
		logrus.Debugf("Seaweedfs installed!")

		in, err := kubeutils.GetLatestInstallation(ctx, kcli)
		if err != nil {
			return errors.Wrap(err, "get latest installation")
		}

		operatorImage, err := getOperatorImage()
		if err != nil {
			return errors.Wrap(err, "get operator image")
		}

		// TODO: timeout

		loading.Infof("Migrating data for high availability")
		logrus.Debugf("Migrating data for high availability")
		progressCh, errCh, err := registrymigrate.RunDataMigrationJob(ctx, kcli, kclient, in, operatorImage)
		if err != nil {
			return errors.Wrap(err, "run registry data migration job")
		}
		if err := waitForJobAndLogProgress(loading, progressCh, errCh); err != nil {
			return errors.Wrap(err, "registry data migration job failed")
		}
		logrus.Debugf("Data migration complete!")

		loading.Infof("Enabling registry high availability")
		logrus.Debugf("Enabling registry high availability")
		err = enableRegistryHA(ctx, kcli, hcli, serviceCIDR, cfgspec)
		if err != nil {
			return errors.Wrap(err, "enable registry high availability")
		}
		logrus.Debugf("Registry high availability enabled!")
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

// enableRegistryHA scales the registry deployment to the desired number of replicas.
func enableRegistryHA(ctx context.Context, kcli client.Client, hcli helm.Client, serviceCIDR string, cfgspec *ecv1beta1.ConfigSpec) error {
	// TODO (@salah): add support for end user overrides
	r := &registry.Registry{
		IsHA:        true,
		ServiceCIDR: serviceCIDR,
	}
	if err := r.Upgrade(ctx, kcli, hcli, addOnOverrides(r, cfgspec, nil)); err != nil {
		return errors.Wrap(err, "upgrade registry")
	}

	return nil
}

// EnableAdminConsoleHA enables high availability for the admin console.
func EnableAdminConsoleHA(ctx context.Context, kcli client.Client, hcli helm.Client, isAirgap bool, serviceCIDR string, proxy *ecv1beta1.ProxySpec, cfgspec *ecv1beta1.ConfigSpec) error {
	// TODO (@salah): add support for end user overrides
	ac := &adminconsole.AdminConsole{
		IsAirgap:    isAirgap,
		IsHA:        true,
		Proxy:       proxy,
		ServiceCIDR: serviceCIDR,
	}
	if err := ac.Upgrade(ctx, kcli, hcli, addOnOverrides(ac, cfgspec, nil)); err != nil {
		return errors.Wrap(err, "upgrade admin console")
	}

	return nil
}

func waitForJobAndLogProgress(progressWriter *spinner.MessageWriter, progressCh <-chan string, errCh <-chan error) error {
	for {
		select {
		case err := <-errCh:
			return err
		case progress := <-progressCh:
			logrus.Debugf("Migrating data for high availability (%s)", progress)
			progressWriter.Infof("Migrating data for high availability (%s)", progress)
		}
	}
}
