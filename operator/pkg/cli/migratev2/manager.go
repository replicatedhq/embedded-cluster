package migratev2

import (
	"context"
	"fmt"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"golang.org/x/sync/errgroup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	jobNamePrefix = "install-v2-manager-"
)

// RunManagerInstallJobsAndWait runs the v2 manager install job and waits for it to finish.
func RunManagerInstallJobsAndWait(
	ctx context.Context, logf LogFunc, cli client.Client,
	installation *ecv1beta1.Installation,
	licenseID string, licenseEndpoint string, versionLabel string,
) error {
	var nodes corev1.NodeList
	if err := cli.List(ctx, &nodes); err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	for _, node := range nodes.Items {
		logf("Ensuring manager install job for node %s", node.Name)
		_, err := ensureManagerInstallJobForNode(ctx, cli, node, installation, licenseID, licenseEndpoint, versionLabel)
		if err != nil {
			return fmt.Errorf("create job for node %s: %w", node.Name, err)
		}
		logf("Successfully ensured manager install job for node %s", node.Name)
	}

	logf("Waiting for manager install jobs to finish")
	err := waitForManagerInstallJobs(ctx, cli, nodes.Items)
	if err != nil {
		return fmt.Errorf("wait for jobs: %w", err)
	}
	logf("Successfully waited for manager install jobs to finish")

	logf("Deleting manager install jobs")
	err = deleteManagerInstallJobs(ctx, cli, nodes.Items)
	if err != nil {
		return fmt.Errorf("delete jobs: %w", err)
	}
	logf("Successfully deleted manager install jobs")

	return nil
}

func ensureManagerInstallJobForNode(
	ctx context.Context, cli client.Client,
	node corev1.Node, installation *ecv1beta1.Installation,
	licenseID string, licenseEndpoint string, versionLabel string,
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
		}
	} else if !k8serrors.IsNotFound(err) {
		return "", fmt.Errorf("get job for node %s: %w", node.Name, err)
	}

	job := getManagerInstallJobSpecForNode(node, installation, licenseID, licenseEndpoint, versionLabel)
	if err := cli.Create(ctx, job); err != nil {
		return "", err
	}

	return job.Name, nil
}

func deleteManagerInstallJobs(ctx context.Context, cli client.Client, nodes []corev1.Node) error {
	for _, node := range nodes {
		jobName := getManagerInstallJobName(node)
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ecNamespace, Name: jobName,
			},
		}
		err := cli.Delete(ctx, job)
		if err != nil {
			return fmt.Errorf("delete job for node %s: %w", node.Name, err)
		}
	}

	return nil
}

func waitForManagerInstallJobs(ctx context.Context, cli client.Client, nodes []corev1.Node) error {
	eg, egCtx := errgroup.WithContext(ctx)

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

	<-egCtx.Done()
	err := egCtx.Err()
	if err != nil {
		return err
	}

	return nil
}

func waitForManagerInstallJob(ctx context.Context, cli client.Client, jobName string) error {
	// 120 steps at 5 second intervals = ~ 10 minutes
	return kubeutils.WaitForJob(ctx, cli, ecNamespace, jobName, 120, 1)
}

func getManagerInstallJobForNode(ctx context.Context, cli client.Client, node corev1.Node) (batchv1.Job, error) {
	jobName := getManagerInstallJobName(node)

	var job batchv1.Job
	err := cli.Get(ctx, client.ObjectKey{Namespace: ecNamespace, Name: jobName}, &job)
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
