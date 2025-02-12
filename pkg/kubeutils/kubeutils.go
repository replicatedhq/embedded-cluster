package kubeutils

import (
	"context"
	"fmt"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KubeUtils struct{}

var _ KubeUtilsInterface = (*KubeUtils)(nil)

func (k *KubeUtils) WaitForNamespace(ctx context.Context, cli client.Client, ns string, opts *WaitOptions) error {
	backoff := opts.GetBackoff()
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			ready, err := k.IsNamespaceReady(ctx, cli, ns)
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
func (k *KubeUtils) WaitForDeployment(ctx context.Context, cli client.Client, ns, name string, opts *WaitOptions) error {
	backoff := opts.GetBackoff()
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			ready, err := k.IsDeploymentReady(ctx, cli, ns, name)
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
func (k *KubeUtils) WaitForDaemonset(ctx context.Context, cli client.Client, ns, name string, opts *WaitOptions) error {
	backoff := opts.GetBackoff()
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			ready, err := k.IsDaemonsetReady(ctx, cli, ns, name)
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

func (k *KubeUtils) WaitForService(ctx context.Context, cli client.Client, ns, name string, opts *WaitOptions) error {
	backoff := opts.GetBackoff()
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

// WaitForJob waits for a job to have a certain number of completions.
func (k *KubeUtils) WaitForJob(ctx context.Context, cli client.Client, ns, name string, completions int32, opts *WaitOptions) error {
	backoff := opts.GetBackoff()
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			var job batchv1.Job
			err := cli.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &job)
			if k8serrors.IsNotFound(err) {
				// exit
				lasterr = fmt.Errorf("job not found")
				return false, lasterr
			} else if err != nil {
				lasterr = fmt.Errorf("unable to get job: %w", err)
				return false, nil
			}

			failed := k.isJobFailed(job)
			if failed {
				// exit
				lasterr = fmt.Errorf("job failed")
				return false, lasterr
			}

			completed := k.isJobCompleted(job, completions)
			if completed {
				return true, nil
			}

			// TODO: need to handle the case where the pod get stuck in pending
			// This can happen if nodes are not schedulable or if a volume is not found

			return false, nil
		},
	); err != nil {
		if lasterr != nil {
			return lasterr
		}
		return fmt.Errorf("timed out waiting for job %s", name)
	}
	return nil
}

// WaitForPod waits for a pod to be completed.
func (k *KubeUtils) WaitForPodComplete(ctx context.Context, cli client.Client, ns, name string, opts *WaitOptions) error {
	backoff := opts.GetBackoff()
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			ready, err := k.IsPodComplete(ctx, cli, ns, name)
			if err != nil {
				lasterr = fmt.Errorf("unable to get pod status: %w", err)
				return false, nil
			}
			return ready, nil
		},
	); err != nil {
		if lasterr != nil {
			return fmt.Errorf("timed out waiting for pod %s: %w", name, lasterr)
		} else {
			return fmt.Errorf("timed out waiting for pod %s", name)
		}
	}
	return nil
}

func (k *KubeUtils) WaitForNodes(ctx context.Context, cli client.Client) error {
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
			return readynodes == len(nodes.Items) && len(nodes.Items) > 0, nil
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
func (k *KubeUtils) WaitForControllerNode(ctx context.Context, kcli client.Client, name string) error {
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

func (k *KubeUtils) IsNamespaceReady(ctx context.Context, cli client.Client, ns string) (bool, error) {
	var namespace corev1.Namespace
	if err := cli.Get(ctx, types.NamespacedName{Name: ns}, &namespace); err != nil {
		return false, err
	}
	return namespace.Status.Phase == corev1.NamespaceActive, nil
}

// IsDeploymentReady returns true if the deployment is ready.
func (k *KubeUtils) IsDeploymentReady(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
	var deploy appsv1.Deployment
	nsn := types.NamespacedName{Namespace: ns, Name: name}
	if err := cli.Get(ctx, nsn, &deploy); err != nil {
		return false, err
	}
	if deploy.Spec.Replicas == nil {
		// Defaults to 1 if the replicas field is nil
		if deploy.Status.ReadyReplicas == 1 {
			return true, nil
		}
		return false, nil
	}
	return deploy.Status.ReadyReplicas == *deploy.Spec.Replicas, nil
}

// IsStatefulSetReady returns true if the statefulset is ready.
func (k *KubeUtils) IsStatefulSetReady(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
	var statefulset appsv1.StatefulSet
	nsn := types.NamespacedName{Namespace: ns, Name: name}
	if err := cli.Get(ctx, nsn, &statefulset); err != nil {
		return false, err
	}
	if statefulset.Spec.Replicas == nil {
		// Defaults to 1 if the replicas field is nil
		if statefulset.Status.ReadyReplicas == 1 {
			return true, nil
		}
		return false, nil
	}
	return statefulset.Status.ReadyReplicas == *statefulset.Spec.Replicas, nil
}

// IsDaemonsetReady returns true if the daemonset is ready.
func (k *KubeUtils) IsDaemonsetReady(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
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

// isJobCompleted returns true if the job has been completed successfully.
func (k *KubeUtils) isJobCompleted(job batchv1.Job, completions int32) bool {
	isSucceeded := job.Status.Succeeded >= completions
	return isSucceeded
}

// isJobFailed if the job has exceeded the backoff limit.
func (k *KubeUtils) isJobFailed(job batchv1.Job) bool {
	backoffLimit := int32(6) // default
	if job.Spec.BackoffLimit != nil {
		backoffLimit = *job.Spec.BackoffLimit
	}
	exceedsBackoffLimit := job.Status.Failed > backoffLimit
	return exceedsBackoffLimit
}

// IsPodComplete returns true if the pod has completed.
func (k *KubeUtils) IsPodComplete(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
	pod := corev1.Pod{}
	nsn := types.NamespacedName{Namespace: ns, Name: name}
	err := cli.Get(ctx, nsn, &pod)
	if err != nil {
		return false, err
	}
	if pod.Status.Phase == corev1.PodSucceeded {
		return true, nil
	} else if pod.Status.Phase == corev1.PodFailed {
		return true, fmt.Errorf("pod failed: %s", pod.Status.Reason)
	}
	return false, nil
}

// WaitForKubernetes waits for all deployments to be ready in kube-system, and returns an error channel.
// if either of them fails to become healthy, an error is returned via the channel.
func (k *KubeUtils) WaitForKubernetes(ctx context.Context, cli client.Client) <-chan error {
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
		close(errch)
		return errch
	}

	errch = make(chan error, len(deps.Items))

	wg := sync.WaitGroup{}
	wg.Add(len(deps.Items))
	go func() {
		wg.Wait()
		close(errch)
	}()

	for _, dep := range deps.Items {
		go func(depName string) {
			defer wg.Done()
			err := k.WaitForDeployment(ctx, cli, "kube-system", depName, nil)
			if err != nil {
				errch <- fmt.Errorf("%s failed to become healthy: %w", depName, err)
			}
		}(dep.Name)
	}

	return errch
}

func (k *KubeUtils) WaitForCRDToBeReady(ctx context.Context, cli client.Client, name string) error {
	backoff := wait.Backoff{Steps: 600, Duration: 100 * time.Millisecond, Factor: 1.0, Jitter: 0.1}
	if err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		newCrd := apiextensionsv1.CustomResourceDefinition{}
		err := cli.Get(ctx, client.ObjectKey{Name: name}, &newCrd)
		if err != nil {
			return false, nil // not ready yet
		}
		for _, cond := range newCrd.Status.Conditions {
			if cond.Type == apiextensionsv1.Established && cond.Status == apiextensionsv1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return err
	}
	return nil
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
