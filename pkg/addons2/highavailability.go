package addons2

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
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
func EnableHA(ctx context.Context, kcli client.Client, isAirgap bool, serviceCIDR string, proxy *ecv1beta1.ProxySpec) error {
	loading := spinner.Start()
	defer loading.Close()

	if isAirgap {
		loading.Infof("Enabling high availability")

		// install the helm chart
		hcli, err := helm.NewHelm(helm.HelmOptions{
			KubeConfig: runtimeconfig.PathToKubeConfig(),
			K0sVersion: versions.K0sVersion,
			AirgapPath: runtimeconfig.EmbeddedClusterChartsSubDir(),
		})
		if err != nil {
			return errors.Wrap(err, "create helm client")
		}

		sw := &seaweedfs.SeaweedFS{
			ServiceCIDR: serviceCIDR,
		}
		if err := sw.Install(ctx, kcli, hcli, nil); err != nil {
			return errors.Wrap(err, "install seaweedfs")
		}

		reg := &registry.Registry{
			ServiceCIDR: serviceCIDR,
			IsHA:        true,
		}
		if err := reg.Migrate(ctx, kcli, loading); err != nil {
			return errors.Wrap(err, "migrate registry data")
		}
		if err := reg.Upgrade(ctx, kcli); err != nil {
			return errors.Wrap(err, "upgrade registry")
		}
	}

	loading.Infof("Updating the Admin Console for high availability")

	ac := &adminconsole.AdminConsole{
		IsAirgap: isAirgap,
		IsHA:     true,
		Proxy:    proxy,
	}
	if err := ac.Upgrade(ctx, kcli); err != nil {
		return errors.Wrap(err, "upgrade admin console")
	}

	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "get latest installation")
	}
	in.Spec.HighAvailability = true

	if err := kubeutils.UpdateInstallation(ctx, kcli, in); err != nil {
		return errors.Wrap(err, "update installation")
	}

	loading.Infof("High availability enabled!")
	return nil
}
