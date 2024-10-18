package kubeutils

import (
	"context"
	"fmt"
	"time"

	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

type ErrNoInstallations struct{}

func (e ErrNoInstallations) Error() string {
	return "no installations found"
}

// BackOffToDuration returns the maximum duration of the provided backoff.
func BackOffToDuration(backoff wait.Backoff) time.Duration {
	var total time.Duration
	duration := backoff.Duration
	for i := 0; i < backoff.Steps; i++ {
		total += duration
		duration = time.Duration(float64(duration) * backoff.Factor)
	}
	return total
}

func WaitForNamespace(ctx context.Context, cli client.Client, ns string) error {
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			ready, err := IsNamespaceReady(ctx, cli, ns)
			if err != nil {
				lasterr = fmt.Errorf("unable to get namespace %s status: %v", ns, err)
				return false, nil
			}
			return ready, nil
		},
	); err != nil {
		if lasterr != nil {
			return fmt.Errorf("timed out waiting for namespace %s: %v", ns, lasterr)
		} else {
			return fmt.Errorf("timed out waiting for namespace %s", ns)
		}
	}
	return nil
}

// WaitForDeployment waits for the provided deployment to be ready.
func WaitForDeployment(ctx context.Context, cli client.Client, ns, name string) error {
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			ready, err := IsDeploymentReady(ctx, cli, ns, name)
			if err != nil {
				lasterr = fmt.Errorf("unable to get deploy %s status: %v", name, err)
				return false, nil
			}
			return ready, nil
		},
	); err != nil {
		if lasterr != nil {
			return fmt.Errorf("timed out waiting for %s to deploy: %v", name, lasterr)
		} else {
			return fmt.Errorf("timed out waiting for %s to deploy", name)
		}
	}
	return nil
}

// WaitForDaemonset waits for the provided daemonset to be ready.
func WaitForDaemonset(ctx context.Context, cli client.Client, ns, name string) error {
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			ready, err := IsDaemonsetReady(ctx, cli, ns, name)
			if err != nil {
				lasterr = fmt.Errorf("unable to get daemonset %s status: %v", name, err)
				return false, nil
			}
			return ready, nil
		},
	); err != nil {
		if lasterr != nil {
			return fmt.Errorf("timed out waiting for %s to deploy: %v", name, lasterr)
		} else {
			return fmt.Errorf("timed out waiting for %s to deploy", name)
		}
	}
	return nil
}

func WaitForService(ctx context.Context, cli client.Client, ns, name string) error {
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			var svc corev1.Service
			nsn := types.NamespacedName{Namespace: ns, Name: name}
			if err := cli.Get(ctx, nsn, &svc); err != nil {
				lasterr = fmt.Errorf("unable to get service %s: %v", name, err)
				return false, nil
			}
			return svc.Spec.ClusterIP != "", nil
		},
	); err != nil {
		if lasterr != nil {
			return fmt.Errorf("timed out waiting for service %s to have an IP: %v", name, lasterr)
		} else {
			return fmt.Errorf("timed out waiting for service %s to have an IP", name)
		}
	}
	return nil
}

func WaitForInstallation(ctx context.Context, cli client.Client, writer *spinner.MessageWriter) error {
	backoff := wait.Backoff{Steps: 60 * 5, Duration: time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error

	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			lastInstall, err := GetLatestInstallation(ctx, cli)
			if err != nil {
				lasterr = fmt.Errorf("unable to get latest installation: %v", err)
				return false, nil
			}

			if writer != nil {
				writeStatusMessage(writer, lastInstall)
			}

			// check the status of the installation
			if lastInstall.Status.State == embeddedclusterv1beta1.InstallationStateInstalled {
				return true, nil
			}
			lasterr = fmt.Errorf("installation state is %q (%q)", lastInstall.Status.State, lastInstall.Status.Reason)

			if lastInstall.Status.State == embeddedclusterv1beta1.InstallationStateFailed {
				return false, fmt.Errorf("installation failed: %s", lastInstall.Status.Reason)
			}

			if lastInstall.Status.State == embeddedclusterv1beta1.InstallationStateHelmChartUpdateFailure {
				return false, fmt.Errorf("helm chart installation failed: %s", lastInstall.Status.Reason)
			}

			return false, nil
		},
	); err != nil {
		if wait.Interrupted(err) {
			if lasterr != nil {
				return fmt.Errorf("timed out waiting for the installation to finish: %v", lasterr)
			} else {
				return fmt.Errorf("timed out waiting for the installation to finish")
			}
		}
		return fmt.Errorf("error waiting for installation: %v", err)
	}
	return nil
}

func ListInstallations(ctx context.Context, cli client.Client) ([]embeddedclusterv1beta1.Installation, error) {
	var list embeddedclusterv1beta1.InstallationList
	err := cli.List(ctx, &list)
	if err != nil {
		return nil, err
	}
	installs := list.Items
	for i := range installs {
		installs[i] = MaybeOverrideInstallationDataDirs(installs[i])
	}
	return installs, nil
}

func GetInstallation(ctx context.Context, cli client.Client, name string) (*embeddedclusterv1beta1.Installation, error) {
	nsn := types.NamespacedName{Name: name}
	var install embeddedclusterv1beta1.Installation
	err := cli.Get(ctx, nsn, &install)
	if err != nil {
		return nil, fmt.Errorf("unable to get installation: %w", err)
	}
	install = MaybeOverrideInstallationDataDirs(install)
	return &install, nil
}

func GetLatestInstallation(ctx context.Context, cli client.Client) (*embeddedclusterv1beta1.Installation, error) {
	installs, err := ListInstallations(ctx, cli)
	if meta.IsNoMatchError(err) {
		// this will happen if the CRD is not yet installed
		return nil, ErrNoInstallations{}
	} else if err != nil {
		return nil, fmt.Errorf("unable to list installations: %v", err)
	}

	if len(installs) == 0 {
		return nil, ErrNoInstallations{}
	}

	// get the latest installation
	return &installs[0], nil
}

const (
	// AnnotationHasDataDirectories is an annotation on the installation object that indicates that
	// it was created by an operator that stored information about the location of the data
	// directories. If this is not set, the operator will update the installation object.
	AnnotationHasDataDirectories = "embedded-cluster.replicated.com/has-data-directories"
)

// MaybeOverrideInstallationDataDirs checks if the installation has an annotation indicating that
// it was created or updated by a version that stored the location of the data directories in the
// installation object. If it is not set, it will set the annotation and update the installation
// object with the old location of the data directories.
func MaybeOverrideInstallationDataDirs(in embeddedclusterv1beta1.Installation) embeddedclusterv1beta1.Installation {
	if ok := in.Annotations[AnnotationHasDataDirectories]; ok == "true" {
		return in
	}
	if in.ObjectMeta.Annotations == nil {
		in.ObjectMeta.Annotations = map[string]string{}
	}
	in.ObjectMeta.Annotations[AnnotationHasDataDirectories] = "true"

	if in.Spec.RuntimeConfig == nil {
		in.Spec.RuntimeConfig = &embeddedclusterv1beta1.RuntimeConfigSpec{}
	}

	// In prior versions, the data directories are not a subdirectory of /var/lib/embedded-cluster.
	in.Spec.RuntimeConfig.K0sDataDirOverride = "/var/lib/k0s"
	in.Spec.RuntimeConfig.OpenEBSDataDirOverride = "/var/openebs"

	return in
}

func writeStatusMessage(writer *spinner.MessageWriter, install *embeddedclusterv1beta1.Installation) {
	if install.Status.State != embeddedclusterv1beta1.InstallationStatePendingChartCreation {
		return
	}

	if install.Spec.Config == nil || install.Spec.Config.Extensions.Helm == nil {
		return
	}
	numDesiredCharts := len(install.Spec.Config.Extensions.Helm.Charts)

	pendingChartsMap := map[string]struct{}{}
	for _, chartName := range install.Status.PendingCharts {
		pendingChartsMap[chartName] = struct{}{}
	}

	numPendingCharts := 0
	for _, ch := range install.Spec.Config.Extensions.Helm.Charts {
		if _, ok := pendingChartsMap[ch.Name]; ok {
			numPendingCharts++
		}
	}
	numCompletedCharts := numDesiredCharts - numPendingCharts

	if numCompletedCharts < numDesiredCharts {
		writer.Infof("Waiting for additional components to be ready (%d/%d)", numCompletedCharts, numDesiredCharts)
	} else {
		writer.Infof("Finalizing additional components")
	}
}

func WaitForHAInstallation(ctx context.Context, cli client.Client) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		default:
			lastInstall, err := GetLatestInstallation(ctx, cli)
			if err != nil {
				return fmt.Errorf("unable to get latest installation: %v", err)
			}
			haStatus := CheckConditionStatus(lastInstall.Status, "HighAvailability")
			if haStatus == metav1.ConditionTrue {
				return nil
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func CheckConditionStatus(inStat embeddedclusterv1beta1.InstallationStatus, conditionName string) metav1.ConditionStatus {
	for _, cond := range inStat.Conditions {
		if cond.Type == conditionName {
			return cond.Status
		}
	}

	return ""
}

func WaitForNodes(ctx context.Context, cli client.Client) error {
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			var nodes corev1.NodeList
			if err := cli.List(ctx, &nodes); err != nil {
				lasterr = fmt.Errorf("unable to list nodes: %v", err)
				return false, nil
			}
			readynodes := 0
			for _, node := range nodes.Items {
				for _, condition := range node.Status.Conditions {
					if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
						readynodes++
					}
				}
			}
			return readynodes == len(nodes.Items), nil
		},
	); err != nil {
		if lasterr != nil {
			return fmt.Errorf("timed out waiting for nodes to be ready: %v", lasterr)
		} else {
			return fmt.Errorf("timed out waiting for nodes to be ready")
		}
	}
	return nil
}

// WaitForControllerNode waits for a specific controller node to be registered with the cluster.
func WaitForControllerNode(ctx context.Context, kcli client.Client, name string) error {
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			var node corev1.Node
			if err := kcli.Get(ctx, client.ObjectKey{Name: name}, &node); err != nil {
				lasterr = fmt.Errorf("unable to get node: %v", err)
				return false, nil
			}
			if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; !ok {
				lasterr = fmt.Errorf("control plane label not found")
				return false, nil
			}
			for _, condition := range node.Status.Conditions {
				if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
					return true, nil
				}
			}
			lasterr = fmt.Errorf("node %s not ready", name)
			return false, nil
		},
	); err != nil {
		if lasterr != nil {
			return fmt.Errorf("timed out waiting for node %s: %w", name, lasterr)
		} else {
			return fmt.Errorf("timed out waiting for node %s", name)
		}
	}
	return nil
}

// WaitForJob waits for a job to have a certain number of completions.
func WaitForJob(ctx context.Context, cli client.Client, ns, name string, maxSteps int, completions int32) error {
	backoff := wait.Backoff{Steps: maxSteps, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			ready, err := IsJobComplete(ctx, cli, ns, name, completions)
			if err != nil {
				lasterr = fmt.Errorf("unable to get job status: %w", err)
				return false, nil
			}
			return ready, nil
		},
	); err != nil {
		if lasterr != nil {
			return fmt.Errorf("timed out waiting for job %s: %w", name, lasterr)
		} else {
			return fmt.Errorf("timed out waiting for job %s", name)
		}
	}
	return nil
}

func IsNamespaceReady(ctx context.Context, cli client.Client, ns string) (bool, error) {
	var namespace corev1.Namespace
	if err := cli.Get(ctx, types.NamespacedName{Name: ns}, &namespace); err != nil {
		return false, err
	}
	return namespace.Status.Phase == corev1.NamespaceActive, nil
}

// IsDeploymentReady returns true if the deployment is ready.
func IsDeploymentReady(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
	var deploy appsv1.Deployment
	nsn := types.NamespacedName{Namespace: ns, Name: name}
	if err := cli.Get(ctx, nsn, &deploy); err != nil {
		return false, err
	}
	if deploy.Spec.Replicas == nil {
		return false, nil
	}
	return deploy.Status.ReadyReplicas == *deploy.Spec.Replicas, nil
}

// IsStatefulSetReady returns true if the statefulset is ready.
func IsStatefulSetReady(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
	var statefulset appsv1.StatefulSet
	nsn := types.NamespacedName{Namespace: ns, Name: name}
	if err := cli.Get(ctx, nsn, &statefulset); err != nil {
		return false, err
	}
	if statefulset.Spec.Replicas == nil {
		return false, nil
	}
	return statefulset.Status.ReadyReplicas == *statefulset.Spec.Replicas, nil
}

// IsDaemonsetReady returns true if the daemonset is ready.
func IsDaemonsetReady(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
	var daemonset appsv1.DaemonSet
	nsn := types.NamespacedName{Namespace: ns, Name: name}
	if err := cli.Get(ctx, nsn, &daemonset); err != nil {
		return false, err
	}
	if daemonset.Status.DesiredNumberScheduled == daemonset.Status.NumberReady {
		return true, nil
	}
	return false, nil
}

// IsJobComplete returns true if the job has been completed successfully.
func IsJobComplete(ctx context.Context, cli client.Client, ns, name string, completions int32) (bool, error) {
	var job batchv1.Job
	nsn := types.NamespacedName{Namespace: ns, Name: name}
	if err := cli.Get(ctx, nsn, &job); err != nil {
		return false, err
	}
	if job.Status.Succeeded >= completions {
		return true, nil
	}
	return false, nil
}

// WaitForKubernetes waits for all deployments to be ready in kube-system, and returns an error channel.
// if either of them fails to become healthy, an error is returned via the channel.
func WaitForKubernetes(ctx context.Context, cli client.Client) <-chan error {
	errch := make(chan error, 1)

	// wait until there is at least one deployment in kube-system
	backoff := wait.Backoff{Steps: 60, Duration: time.Second, Factor: 1.0, Jitter: 0.1}
	deps := appsv1.DeploymentList{}
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			if err := cli.List(ctx, &deps, client.InNamespace("kube-system")); err != nil {
				return false, nil
			}
			return len(deps.Items) >= 3, nil // coredns, metrics-server, and calico-kube-controllers
		}); err != nil {
		errch <- fmt.Errorf("timed out waiting for deployments in kube-system: %w", err)
		return errch
	}

	errch = make(chan error, len(deps.Items))

	for _, dep := range deps.Items {
		go func(depName string) {
			err := WaitForDeployment(ctx, cli, "kube-system", depName)
			if err != nil {
				errch <- fmt.Errorf("%s failed to become healthy: %w", depName, err)
			}
		}(dep.Name)
	}

	return errch
}

func NumOfControlPlaneNodes(ctx context.Context, cli client.Client) (int, error) {
	opts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(
			labels.Set{"node-role.kubernetes.io/control-plane": "true"},
		),
	}
	var nodes corev1.NodeList
	if err := cli.List(ctx, &nodes, opts); err != nil {
		return 0, err
	}
	return len(nodes.Items), nil
}
