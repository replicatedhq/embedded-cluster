package addons

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	registrymigrate "github.com/replicatedhq/embedded-cluster/pkg/addons/registry/migrate"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	} else if client.IgnoreNotFound(err) != nil {
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

	if isAirgap {
		loading.Infof("Enabling high availability")
		logrus.Debugf("Enabling high availability")

		hasMigrated, err := registry.IsRegistryHA(ctx, kcli)
		if err != nil {
			return errors.Wrap(err, "check if registry data has been migrated")
		} else if !hasMigrated {
			logrus.Debugf("Installing seaweedfs")
			err = ensureSeaweedfs(ctx, kcli, hcli, serviceCIDR, cfgspec)
			if err != nil {
				return errors.Wrap(err, "ensure seaweedfs")
			}
			logrus.Debugf("Seaweedfs installed!")

			// if the migration fails, we need to scale the registry back to 1
			success := false
			// This must be in a function so that success is evaluated correctly
			defer func() {
				scaleRegistryBackOnFailure(kcli, success)
			}()

			logrus.Debugf("Scaling registry to 0 replicas")
			err := scaleRegistry(ctx, kcli, 0)
			if err != nil {
				return errors.Wrap(err, "scale registry to 0 replicas")
			}

			loading.Infof("Migrating data for high availability")
			logrus.Debugf("Migrating data for high availability")
			err = migrateRegistryData(ctx, kcli, kclient, cfgspec, loading)
			if err != nil {
				return errors.Wrap(err, "migrate registry data")
			}
			logrus.Debugf("Data migration complete!")

			success = true

			loading.Infof("Enabling registry high availability")
			logrus.Debugf("Enabling registry high availability")
			err = enableRegistryHA(ctx, kcli, hcli, serviceCIDR, cfgspec)
			if err != nil {
				return errors.Wrap(err, "enable registry high availability")
			}
			logrus.Debugf("Registry high availability enabled!")
		}
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

func scaleRegistryBackOnFailure(kcli client.Client, success bool) {
	r := recover()

	if !success {
		logrus.Debugf("Scaling registry back to 1 replica after migration failure")

		// this should use the background context as we want it to run even if the context expired
		err := scaleRegistry(context.Background(), kcli, 1)
		if err != nil {
			logrus.Errorf("Failed to scale registry back to 1 replica: %v", err)
		}
	}

	if r != nil {
		panic(r)
	}
}

// scaleRegistry scales the registry deployment to the given replica count.
// '0' and '1' are the only acceptable values.
func scaleRegistry(ctx context.Context, cli client.Client, scale int32) error {
	if scale != 0 && scale != 1 {
		return fmt.Errorf("invalid scale: %d", scale)
	}

	currentRegistry := &appsv1.Deployment{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: runtimeconfig.RegistryNamespace, Name: "registry"}, currentRegistry)
	if err != nil {
		return fmt.Errorf("get registry deployment: %w", err)
	}

	currentRegistry.Spec.Replicas = &scale

	err = cli.Update(ctx, currentRegistry)
	if err != nil {
		return fmt.Errorf("update registry deployment: %w", err)
	}

	return nil
}

// migrateRegistryData runs the registry data migration.
func migrateRegistryData(ctx context.Context, kcli client.Client, kclient kubernetes.Interface, cfgspec *ecv1beta1.ConfigSpec, progressWriter *spinner.MessageWriter) error {
	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "get latest installation")
	}

	operatorImage, err := getOperatorImage()
	if err != nil {
		return errors.Wrap(err, "get operator image")
	}
	domains := runtimeconfig.GetDomains(cfgspec)
	if domains.ProxyRegistryDomain != "" {
		operatorImage = strings.Replace(operatorImage, "proxy.replicated.com", domains.ProxyRegistryDomain, 1)
	}

	// TODO: timeout

	progressCh, errCh, err := registrymigrate.RunDataMigration(ctx, kcli, kclient, in, operatorImage)
	if err != nil {
		return errors.Wrap(err, "run registry data migration")
	}
	if err := waitForPodAndLogProgress(progressWriter, progressCh, errCh); err != nil {
		return errors.Wrap(err, "registry data migration failed")
	}

	return nil
}

// ensureSeaweedfs ensures that seaweedfs is installed.
func ensureSeaweedfs(ctx context.Context, kcli client.Client, hcli helm.Client, serviceCIDR string, cfgspec *ecv1beta1.ConfigSpec) error {
	domains := runtimeconfig.GetDomains(cfgspec)

	// TODO (@salah): add support for end user overrides
	sw := &seaweedfs.SeaweedFS{
		ServiceCIDR:         serviceCIDR,
		ProxyRegistryDomain: domains.ProxyRegistryDomain,
	}

	if err := sw.Uninstall(ctx, kcli); err != nil {
		return errors.Wrap(err, "uninstall seaweedfs")
	}

	if err := sw.Install(ctx, kcli, hcli, addOnOverrides(sw, cfgspec, nil), nil); err != nil {
		return errors.Wrap(err, "install seaweedfs")
	}

	return nil
}

// enableRegistryHA enables high availability for the registry and scales the registry deployment
// to the desired number of replicas.
func enableRegistryHA(ctx context.Context, kcli client.Client, hcli helm.Client, serviceCIDR string, cfgspec *ecv1beta1.ConfigSpec) error {
	domains := runtimeconfig.GetDomains(cfgspec)

	// TODO (@salah): add support for end user overrides
	r := &registry.Registry{
		ServiceCIDR:         serviceCIDR,
		ProxyRegistryDomain: domains.ProxyRegistryDomain,
		IsHA:                true,
	}
	if err := r.Upgrade(ctx, kcli, hcli, addOnOverrides(r, cfgspec, nil)); err != nil {
		return errors.Wrap(err, "upgrade registry")
	}

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

func waitForPodAndLogProgress(progressWriter *spinner.MessageWriter, progressCh <-chan string, errCh <-chan error) error {
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
