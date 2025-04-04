package addons

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
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
func EnableHA(
	ctx context.Context, kcli client.Client, kclient kubernetes.Interface, hcli helm.Client,
	isAirgap bool, serviceCIDR string, proxy *ecv1beta1.ProxySpec, cfgspec *ecv1beta1.ConfigSpec,
	spinner *spinner.MessageWriter,
) error {
	if isAirgap {
		logrus.Debugf("Enabling high availability")
		spinner.Infof("Enabling high availability")

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

			logrus.Debugf("Scaling registry to 0 replicas")
			// if the migration fails, we need to scale the registry back to 1
			defer maybeScaleRegistryBackOnFailure(kcli)
			err := scaleRegistryDown(ctx, kcli)
			if err != nil {
				return errors.Wrap(err, "scale registry to 0 replicas")
			}

			logrus.Debugf("Migrating data for high availability")
			spinner.Infof("Migrating data for high availability")
			err = migrateRegistryData(ctx, kcli, kclient, cfgspec, spinner)
			if err != nil {
				return errors.Wrap(err, "migrate registry data")
			}
			logrus.Debugf("Data migration complete!")

			logrus.Debugf("Enabling high availability for the registry")
			spinner.Infof("Enabling high availability for the registry")
			err = enableRegistryHA(ctx, kcli, hcli, serviceCIDR, cfgspec)
			if err != nil {
				return errors.Wrap(err, "enable registry high availability")
			}
			logrus.Debugf("Registry high availability enabled!")
		}
	}

	logrus.Debugf("Updating the Admin Console for high availability")
	spinner.Infof("Updating the Admin Console for high availability")
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
	spinner.Infof("High availability enabled!")
	return nil
}

// maybeScaleRegistryBackOnFailure scales the registry back to 1 replica if the migration failed
// (the registry is at 0 replicas).
func maybeScaleRegistryBackOnFailure(kcli client.Client) {
	r := recover()

	deploy := &appsv1.Deployment{}
	// this should use the background context as we want it to run even if the context expired
	err := kcli.Get(context.Background(), client.ObjectKey{Namespace: runtimeconfig.RegistryNamespace, Name: "registry"}, deploy)
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

	err = kcli.Update(context.Background(), deploy)
	if err != nil {
		logrus.Errorf("Failed to scale registry back to 1 replica: %v", err)
	}

	if r != nil {
		panic(r)
	}
}

// scaleRegistryDown scales the registry deployment to 0 replicas.
func scaleRegistryDown(ctx context.Context, cli client.Client) error {
	deploy := &appsv1.Deployment{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: runtimeconfig.RegistryNamespace, Name: "registry"}, deploy)
	if err != nil {
		return fmt.Errorf("get registry deployment: %w", err)
	}

	deploy.Spec.Replicas = ptr.To(int32(0))

	err = cli.Update(ctx, deploy)
	if err != nil {
		return fmt.Errorf("update registry deployment: %w", err)
	}

	return nil
}

// migrateRegistryData runs the registry data migration.
func migrateRegistryData(ctx context.Context, kcli client.Client, kclient kubernetes.Interface, cfgspec *ecv1beta1.ConfigSpec, writer *spinner.MessageWriter) error {
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
	if err := waitForPodAndLogProgress(writer, progressCh, errCh); err != nil {
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

	if err := sw.Upgrade(ctx, kcli, hcli, addOnOverrides(sw, cfgspec, nil)); err != nil {
		return errors.Wrap(err, "upgrade seaweedfs")
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

	if err := kubeutils.WaitForStatefulset(ctx, kcli, runtimeconfig.KotsadmNamespace, "kotsadm-rqlite", nil); err != nil {
		return errors.Wrap(err, "wait for rqlite to be ready")
	}

	// get the list of rqlite pods
	pods := &corev1.PodList{}
	err := kcli.List(ctx, pods, client.InNamespace(runtimeconfig.KotsadmNamespace), client.MatchingLabels{"app": "kotsadm-rqlite"})
	if err != nil {
		return errors.Wrap(err, "list rqlite pods")
	}

	logrus.Infof("Waiting for %d rqlite pods to be ready", len(pods.Items))

	wg := sync.WaitGroup{}
	wg.Add(len(pods.Items))
	isFailed := false

	// wait for all rqlite pods to return healthy responses to ip:4001/readyz?sync&timeout=5s
	for _, pod := range pods.Items {
		go func(pod corev1.Pod) {
			if err := waitForRqliteSync(ctx, pod); err != nil {
				logrus.Errorf("rqlite pod %s failed to sync: %v", pod.Name, err)
				isFailed = true
			}
			wg.Done()
		}(pod)
	}

	wg.Wait()
	if isFailed {
		return fmt.Errorf("rqlite pods failed to sync")
	}
	return nil
}

func waitForPodAndLogProgress(writer *spinner.MessageWriter, progressCh <-chan string, errCh <-chan error) error {
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

func waitForRqliteSync(ctx context.Context, pod corev1.Pod) error {
	err := wait.ExponentialBackoffWithContext(ctx, wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1.0,
		Steps:    60,
	}, func(ctx context.Context) (bool, error) {
		url := fmt.Sprintf("http://%s:4001/readyz?sync&timeout=5s", pod.Status.PodIP)
		resp, err := http.Get(url)
		if err != nil {
			logrus.Errorf("rqlite pod %s returned error: %v", pod.Name, err)
			return false, nil
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			logrus.Errorf("rqlite pod %s returned status code %d", pod.Name, resp.StatusCode)
			return false, nil
		}

		// read the body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logrus.Errorf("rqlite pod %s returned error reading body: %v", pod.Name, err)
			return false, nil
		}

		// look for 'sync ok' in the body
		if strings.Contains(string(body), "sync ok") {
			return true, nil
		}

		logrus.Infof("rqlite pod %s returned %q", pod.Name, string(body))

		return false, nil
	})
	if err != nil {
		return errors.Wrap(err, "wait for rqlite to be ready")
	}
	return nil
}
