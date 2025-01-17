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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	jobNamespace  = "embedded-cluster"
	jobNamePrefix = "install-v2-manager-"
)

// runManagerInstallJobsAndWait runs the v2 manager install job on all nodes and waits for the jobs
// to finish.
func runManagerInstallJobsAndWait(
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
		logf("Ensuring manager install job for node %s", node.Name)
		_, err := ensureManagerInstallJobForNode(ctx, cli, node, in, operatorImage, migrationSecret, appSlug, appVersionLabel)
		if err != nil {
			return fmt.Errorf("create job for node %s: %w", node.Name, err)
		}
		logf("Successfully ensured manager install job for node %s", node.Name)
	}

	logf("Waiting for manager install jobs to finish")
	err = waitForManagerInstallJobs(ctx, cli, nodeList.Items)
	if err != nil {
		return fmt.Errorf("wait for jobs: %w", err)
	}
	logf("Successfully waited for manager install jobs to finish")

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

func ensureManagerInstallJobForNode(
	ctx context.Context, cli client.Client,
	node corev1.Node, in *ecv1beta1.Installation, operatorImage string,
	migrationSecret string, appSlug string, appVersionLabel string,
) (string, error) {
	existing, err := getManagerInstallJobForNode(ctx, cli, node)
	if err == nil {
		if managerInstallJobHasSucceeded(existing) {
			return existing.Name, nil
		} else if managerInstallJobHasFailed(existing) {
			err := cli.Delete(ctx, &existing)
			if err != nil {
				return "", fmt.Errorf("delete job %s: %w", existing.Name, err)
			}
		} else {
			// still running
			return existing.Name, nil
		}
	} else if !k8serrors.IsNotFound(err) {
		return "", fmt.Errorf("get job for node %s: %w", node.Name, err)
	}

	job := getManagerInstallJobSpecForNode(node, in, operatorImage, migrationSecret, appSlug, appVersionLabel)
	if err := cli.Create(ctx, job); err != nil {
		return "", err
	}

	return job.Name, nil
}

// deleteManagerInstallJobs deletes all manager install jobs on all nodes.
func deleteManagerInstallJobs(ctx context.Context, logf LogFunc, cli client.Client) error {
	logf("Deleting manager install jobs")

	var nodeList corev1.NodeList
	if err := cli.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	for _, node := range nodeList.Items {
		jobName := getManagerInstallJobName(node)
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: jobNamespace, Name: jobName,
			},
		}
		err := cli.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil {
			return fmt.Errorf("delete job for node %s: %w", node.Name, err)
		}
	}

	logf("Successfully deleted manager install jobs")

	return nil
}

func waitForManagerInstallJobs(ctx context.Context, cli client.Client, nodes []corev1.Node) error {
	eg := errgroup.Group{}

	for _, node := range nodes {
		jobName := getManagerInstallJobName(node)
		eg.Go(func() error {
			err := waitForManagerInstallJob(ctx, cli, jobName)
			if err != nil {
				return fmt.Errorf("wait for job for node %s: %v", node.Name, err)
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

func waitForManagerInstallJob(ctx context.Context, cli client.Client, jobName string) error {
	// 60 steps at 5 second intervals = ~ 5 minutes
	backoff := wait.Backoff{Steps: 60, Duration: 2 * time.Second, Factor: 1.0, Jitter: 0.1}
	return kubeutils.WaitForJob(ctx, cli, jobNamespace, jobName, 1, &kubeutils.WaitOptions{Backoff: &backoff})
}

func getManagerInstallJobForNode(ctx context.Context, cli client.Client, node corev1.Node) (batchv1.Job, error) {
	jobName := getManagerInstallJobName(node)

	var job batchv1.Job
	err := cli.Get(ctx, client.ObjectKey{Namespace: jobNamespace, Name: jobName}, &job)
	if err != nil {
		return job, fmt.Errorf("get job %s: %w", jobName, err)
	}
	return job, nil
}

func managerInstallJobHasSucceeded(job batchv1.Job) bool {
	return job.Status.Succeeded > 0
}

func managerInstallJobHasFailed(job batchv1.Job) bool {
	return job.Status.Failed > 0
}
