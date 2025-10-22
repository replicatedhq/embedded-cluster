package addons

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	registrymigrate "github.com/replicatedhq/embedded-cluster/pkg/addons/registry/migrate"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EnableHAOptions struct {
	ClusterID          string
	AdminConsolePort   int
	IsAirgap           bool
	IsMultiNodeEnabled bool
	EmbeddedConfigSpec *ecv1beta1.ConfigSpec
	EndUserConfigSpec  *ecv1beta1.ConfigSpec
	ProxySpec          *ecv1beta1.ProxySpec
	HostCABundlePath   string
	DataDir            string
	K0sDataDir         string
	SeaweedFSDataDir   string
	ServiceCIDR        string
	KotsadmNamespace   string
}

// CanEnableHA checks if high availability can be enabled in the cluster.
func (a *AddOns) CanEnableHA(ctx context.Context) (bool, string, error) {
	in, err := kubeutils.GetLatestInstallation(ctx, a.kcli)
	if err != nil {
		return false, "", errors.Wrap(err, "get latest installation")
	}
	if in.Spec.HighAvailability {
		return false, "already enabled", nil
	}

	if err := a.kcli.Get(ctx, client.ObjectKey{Name: constants.EcRestoreStateCMName, Namespace: "embedded-cluster"}, &corev1.ConfigMap{}); err == nil {
		return false, "a restore is in progress", nil
	} else if client.IgnoreNotFound(err) != nil {
		return false, "", errors.Wrap(err, "get restore state configmap")
	}

	ncps, err := kubeutils.NumOfControlPlaneNodes(ctx, a.kcli)
	if err != nil {
		return false, "", errors.Wrap(err, "check control plane nodes")
	}
	if ncps < 3 {
		return false, "number of control plane nodes is less than 3", nil
	}
	return true, "", nil
}

// EnableHA enables high availability.
func (a *AddOns) EnableHA(ctx context.Context, opts EnableHAOptions, spinner *spinner.MessageWriter) error {
	if opts.IsAirgap {
		logrus.Debugf("Enabling high availability")
		spinner.Infof("Enabling high availability")

		hasMigrated, err := registry.IsRegistryHA(ctx, a.kcli)
		if err != nil {
			return errors.Wrap(err, "check if registry data has been migrated")
		} else if !hasMigrated {
			logrus.Debugf("Installing seaweedfs")
			err = a.ensureSeaweedfs(ctx, opts)
			if err != nil {
				return errors.Wrap(err, "ensure seaweedfs")
			}
			logrus.Debugf("Seaweedfs installed!")

			logrus.Debugf("Scaling registry to 0 replicas")
			// if the migration fails, we need to scale the registry back to 1
			defer a.maybeScaleRegistryBackOnFailure()
			err := a.scaleRegistryDown(ctx)
			if err != nil {
				return errors.Wrap(err, "scale registry to 0 replicas")
			}

			logrus.Debugf("Migrating data for high availability")
			spinner.Infof("Migrating data for high availability")
			err = a.migrateRegistryData(ctx, opts.EmbeddedConfigSpec, spinner)
			if err != nil {
				return errors.Wrap(err, "migrate registry data")
			}
			logrus.Debugf("Data migration complete!")

			logrus.Debugf("Enabling high availability for the registry")
			spinner.Infof("Enabling high availability for the registry")
			err = a.enableRegistryHA(ctx, opts)
			if err != nil {
				return errors.Wrap(err, "enable registry high availability")
			}
			logrus.Debugf("Registry high availability enabled!")
		}
	}

	logrus.Debugf("Updating the Admin Console for high availability")
	spinner.Infof("Updating the Admin Console for high availability")
	err := a.EnableAdminConsoleHA(ctx, opts)
	if err != nil {
		return errors.Wrap(err, "enable admin console high availability")
	}
	logrus.Debugf("Admin console high availability enabled!")

	in, err := kubeutils.GetLatestInstallation(ctx, a.kcli)
	if err != nil {
		return errors.Wrap(err, "get latest installation")
	}

	if err := kubeutils.UpdateInstallation(ctx, a.kcli, in, func(in *ecv1beta1.Installation) {
		in.Spec.HighAvailability = true
	}); err != nil {
		return errors.Wrap(err, "update installation")
	}

	logrus.Debugf("High availability enabled!")
	spinner.Infof("High availability enabled!")
	return nil
}

// maybeScaleRegistryBackOnFailure scales the registry back to 1 replica if the migration failed
// (the registry is at 0 replicas).
func (a *AddOns) maybeScaleRegistryBackOnFailure() {
	r := recover()

	deploy := &appsv1.Deployment{}
	// this should use the background context as we want it to run even if the context expired
	err := a.kcli.Get(context.Background(), client.ObjectKey{Namespace: constants.RegistryNamespace, Name: "registry"}, deploy)
	if err != nil {
		logrus.Errorf("Failed to get registry deployment: %v", err)
		return
	}

	// if the deployment is already scaled up, it probably means success
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas > 0 {
		return
	}

	logrus.Debugf("Scaling registry back to 1 replica after migration failure")

	deploy.Spec.Replicas = ptr.To[int32](1)

	err = a.kcli.Update(context.Background(), deploy)
	if err != nil {
		logrus.Errorf("Failed to scale registry back to 1 replica: %v", err)
	}

	if r != nil {
		panic(r)
	}
}

// scaleRegistryDown scales the registry deployment to 0 replicas.
func (a *AddOns) scaleRegistryDown(ctx context.Context) error {
	deploy := &appsv1.Deployment{}
	err := a.kcli.Get(ctx, client.ObjectKey{Namespace: constants.RegistryNamespace, Name: "registry"}, deploy)
	if err != nil {
		return fmt.Errorf("get registry deployment: %w", err)
	}

	deploy.Spec.Replicas = ptr.To(int32(0))

	err = a.kcli.Update(ctx, deploy)
	if err != nil {
		return fmt.Errorf("update registry deployment: %w", err)
	}

	return nil
}

// migrateRegistryData runs the registry data migration.
func (a *AddOns) migrateRegistryData(ctx context.Context, cfgspec *ecv1beta1.ConfigSpec, writer *spinner.MessageWriter) error {
	in, err := kubeutils.GetLatestInstallation(ctx, a.kcli)
	if err != nil {
		return errors.Wrap(err, "get latest installation")
	}

	operatorImage, err := getOperatorImage()
	if err != nil {
		return errors.Wrap(err, "get operator image")
	}
	if a.domains.ProxyRegistryDomain != "" {
		operatorImage = strings.Replace(operatorImage, "proxy.replicated.com", a.domains.ProxyRegistryDomain, 1)
	}

	// TODO: timeout

	progressCh, errCh, err := registrymigrate.RunDataMigration(ctx, a.kcli, a.kclient, in, operatorImage)
	if err != nil {
		return errors.Wrap(err, "run registry data migration")
	}
	if err := a.waitForPodAndLogProgress(writer, progressCh, errCh); err != nil {
		return errors.Wrap(err, "registry data migration failed")
	}

	return nil
}

// ensureSeaweedfs ensures that seaweedfs is installed.
func (a *AddOns) ensureSeaweedfs(ctx context.Context, opts EnableHAOptions) error {
	// TODO (@salah): add support for end user overrides
	sw := &seaweedfs.SeaweedFS{
		ServiceCIDR:      opts.ServiceCIDR,
		SeaweedFSDataDir: opts.SeaweedFSDataDir,
	}

	if err := sw.Upgrade(ctx, a.logf, a.kcli, a.mcli, a.hcli, a.domains, a.addOnOverrides(sw, opts.EmbeddedConfigSpec, opts.EndUserConfigSpec)); err != nil {
		return errors.Wrap(err, "upgrade seaweedfs")
	}

	return nil
}

// enableRegistryHA enables high availability for the registry and scales the registry deployment
// to the desired number of replicas.
func (a *AddOns) enableRegistryHA(ctx context.Context, opts EnableHAOptions) error {
	// TODO (@salah): add support for end user overrides
	r := &registry.Registry{
		ServiceCIDR: opts.ServiceCIDR,
		IsHA:        true,
	}
	if err := r.Upgrade(ctx, a.logf, a.kcli, a.mcli, a.hcli, a.domains, a.addOnOverrides(r, opts.EmbeddedConfigSpec, opts.EndUserConfigSpec)); err != nil {
		return errors.Wrap(err, "upgrade registry")
	}

	return nil
}

// EnableAdminConsoleHA enables high availability for the admin console.
func (a *AddOns) EnableAdminConsoleHA(ctx context.Context, opts EnableHAOptions) error {
	// TODO (@salah): add support for end user overrides
	ac := &adminconsole.AdminConsole{
		ClusterID:          opts.ClusterID,
		IsAirgap:           opts.IsAirgap,
		IsHA:               true,
		Proxy:              opts.ProxySpec,
		ServiceCIDR:        opts.ServiceCIDR,
		IsMultiNodeEnabled: opts.IsMultiNodeEnabled,
		HostCABundlePath:   opts.HostCABundlePath,
		DataDir:            opts.DataDir,
		K0sDataDir:         opts.K0sDataDir,
		AdminConsolePort:   opts.AdminConsolePort,
		KotsadmNamespace:   opts.KotsadmNamespace,
	}
	if err := ac.Upgrade(ctx, a.logf, a.kcli, a.mcli, a.hcli, a.domains, a.addOnOverrides(ac, opts.EmbeddedConfigSpec, opts.EndUserConfigSpec)); err != nil {
		return errors.Wrap(err, "upgrade admin console")
	}

	if err := kubeutils.WaitForStatefulset(ctx, a.kcli, constants.KotsadmNamespace, "kotsadm-rqlite", nil); err != nil {
		return errors.Wrap(err, "wait for rqlite to be ready")
	}

	return nil
}

func (a *AddOns) waitForPodAndLogProgress(writer *spinner.MessageWriter, progressCh <-chan string, errCh <-chan error) error {
	for {
		select {
		case err := <-errCh:
			return err
		case progress := <-progressCh:
			logrus.Debugf("Migrating data for high availability (%s)", progress)
			writer.Infof("Migrating data for high availability (%s)", progress)
		}
	}
}
