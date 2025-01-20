package migratev2

import (
	"context"
	"fmt"
	"strings"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/metadata"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	podNamespace  = "embedded-cluster"
	podNamePrefix = "install-v2-manager-"
)

// runManagerInstallPodsAndWait runs the v2 manager install pod on all nodes and waits for the pods
// to finish.
func runManagerInstallPodsAndWait(
	ctx context.Context, logf LogFunc, cli client.Client,
	in *ecv1beta1.Installation,
	migrationSecret string, appSlug string, appVersionLabel string,
) error {
	logf("Ensuring installation config map")
	if err := ensureInstallationConfigMap(ctx, cli, in); err != nil {
		return fmt.Errorf("ensure installation config map: %w", err)
	}
	logf("Successfully ensured installation config map")

	logf("Getting operator image name")
	operatorImage, err := getOperatorImageName(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("get operator image name: %w", err)
	}
	logf("Successfully got operator image name")

	var nodeList corev1.NodeList
	if err := cli.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	for _, node := range nodeList.Items {
		logf("Ensuring manager install pod for node %s", node.Name)
		_, err := ensureManagerInstallPodForNode(ctx, cli, node, in, operatorImage, migrationSecret, appSlug, appVersionLabel)
		if err != nil {
			return fmt.Errorf("create pod for node %s: %w", node.Name, err)
		}
		logf("Successfully ensured manager install pod for node %s", node.Name)
	}

	logf("Waiting for manager install pods to finish")
	err = waitForManagerInstallPods(ctx, cli, nodeList.Items)
	if err != nil {
		return fmt.Errorf("wait for pods: %w", err)
	}
	logf("Successfully waited for manager install pods to finish")

	return nil
}

func getOperatorImageName(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) (string, error) {
	if in.Spec.AirGap {
		err := metadata.CopyVersionMetadataToCluster(ctx, cli, in)
		if err != nil {
			return "", fmt.Errorf("copy version metadata to cluster: %w", err)
		}
	}

	meta, err := release.MetadataFor(ctx, in, cli)
	if err != nil {
		return "", fmt.Errorf("get release metadata: %w", err)
	}

	for _, image := range meta.Images {
		if strings.Contains(image, "embedded-cluster-operator-image") {
			return image, nil
		}
	}
	return "", fmt.Errorf("no embedded-cluster-operator image found in release metadata")
}

func ensureManagerInstallPodForNode(
	ctx context.Context, cli client.Client,
	node corev1.Node, in *ecv1beta1.Installation, operatorImage string,
	migrationSecret string, appSlug string, appVersionLabel string,
) (string, error) {
	existing, err := getManagerInstallPodForNode(ctx, cli, node)
	if err == nil {
		if managerInstallPodHasSucceeded(existing) {
			return existing.Name, nil
		} else if managerInstallPodHasFailed(existing) {
			err := cli.Delete(ctx, &existing)
			if err != nil {
				return "", fmt.Errorf("delete pod %s: %w", existing.Name, err)
			}
		} else {
			// still running
			return existing.Name, nil
		}
	} else if !k8serrors.IsNotFound(err) {
		return "", fmt.Errorf("get pod for node %s: %w", node.Name, err)
	}

	pod := getManagerInstallPodSpecForNode(node, in, operatorImage, migrationSecret, appSlug, appVersionLabel)
	if err := cli.Create(ctx, pod); err != nil {
		return "", err
	}

	return pod.Name, nil
}

// deleteManagerInstallPods deletes all manager install pods on all nodes.
func deleteManagerInstallPods(ctx context.Context, logf LogFunc, cli client.Client) error {
	logf("Deleting manager install pods")

	var nodeList corev1.NodeList
	if err := cli.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	for _, node := range nodeList.Items {
		podName := getManagerInstallPodName(node)
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: podNamespace, Name: podName,
			},
		}
		err := cli.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil {
			return fmt.Errorf("delete pod for node %s: %w", node.Name, err)
		}
	}

	logf("Successfully deleted manager install pods")

	return nil
}

func waitForManagerInstallPods(ctx context.Context, cli client.Client, nodes []corev1.Node) error {
	eg := errgroup.Group{}

	for _, node := range nodes {
		podName := getManagerInstallPodName(node)
		eg.Go(func() error {
			err := waitForManagerInstallPod(ctx, cli, podName)
			if err != nil {
				return fmt.Errorf("wait for pod for node %s: %v", node.Name, err)
			}
			return nil
		})
	}

	// wait cancels
	err := eg.Wait()
	if err != nil {
		return err
	}

	return nil
}

func waitForManagerInstallPod(ctx context.Context, cli client.Client, podName string) error {
	// 60 steps at 5 second intervals = ~ 5 minutes
	backoff := wait.Backoff{Steps: 60, Duration: 2 * time.Second, Factor: 1.0, Jitter: 0.1}
	return kubeutils.WaitForPodComplete(ctx, cli, podNamespace, podName, &kubeutils.WaitOptions{Backoff: &backoff})
}

func getManagerInstallPodForNode(ctx context.Context, cli client.Client, node corev1.Node) (corev1.Pod, error) {
	podName := getManagerInstallPodName(node)

	var pod corev1.Pod
	err := cli.Get(ctx, client.ObjectKey{Namespace: podNamespace, Name: podName}, &pod)
	if err != nil {
		return pod, fmt.Errorf("get pod %s: %w", podName, err)
	}
	return pod, nil
}

func managerInstallPodHasSucceeded(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded
}

func managerInstallPodHasFailed(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodFailed
}
