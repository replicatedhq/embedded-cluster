package artifacts

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	operatorartifacts "github.com/replicatedhq/embedded-cluster/operator/pkg/artifacts"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DistributeArtifacts distributes artifacts to all nodes and, for airgap installations,
// ensures artifacts are loaded into the cluster.
func DistributeArtifacts(
	ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation,
	localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion string,
) error {
	// let's make sure all assets have been copied to nodes.
	// this may take some time so we only move forward when 'ready'.
	err := ensureArtifactsOnNodes(ctx, cli, rc, in, localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion)
	if err != nil {
		return fmt.Errorf("ensure artifacts: %w", err)
	}

	if in.Spec.AirGap {
		// once all assets are in place we can create the autopilot plan to push the images to
		// containerd.
		err := ensureAirgapArtifactsInCluster(ctx, cli, rc, in)
		if err != nil {
			return fmt.Errorf("autopilot copy airgap artifacts: %w", err)
		}
	}

	return nil
}

func ensureArtifactsOnNodes(
	ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation,
	localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion string,
) error {
	log := controllerruntime.LoggerFrom(ctx)

	log.Info("Placing artifacts on nodes...")

	if in.Spec.AirGap {
		op, err := operatorartifacts.EnsureRegistrySecretInECNamespace(ctx, cli, in)
		if err != nil {
			return fmt.Errorf("ensure registry secret in ec namespace: %w", err)
		} else if op != controllerutil.OperationResultNone {
			log.Info("Registry credentials secret changed", "operation", op)
		}
	}

	err := operatorartifacts.EnsureArtifactsJobForNodes(ctx, cli, rc, in, localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion)
	if err != nil {
		return fmt.Errorf("ensure artifacts job for nodes: %w", err)
	}

	log.Info("Waiting for artifacts to be placed on nodes...")

	err = wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		jobs, err := operatorartifacts.ListArtifactsJobForNodes(ctx, cli, in)
		if err != nil {
			return false, fmt.Errorf("list artifacts jobs for nodes: %w", err)
		}

		ready := true
		for nodeName, job := range jobs {
			if job == nil {
				return false, fmt.Errorf("job for node %s not found", nodeName)
			}
			if job.Status.Succeeded > 0 {
				continue
			}
			ready = false
			for _, cond := range job.Status.Conditions {
				if cond.Type == batchv1.JobFailed {
					if cond.Status == corev1.ConditionTrue {
						// fail immediately if any job fails
						return false, fmt.Errorf("job for node %s failed: %s - %s", nodeName, cond.Reason, cond.Message)
					}
					break
				}
			}
			// job is still running
		}

		return ready, nil
	})
	if err != nil {
		return fmt.Errorf("wait for artifacts job for nodes: %w", err)
	}

	log.Info("Artifacts placed on nodes")

	// cleanup the jobs and the secret created by EnsureArtifactsJobForNodes
	if err := operatorartifacts.CleanupArtifactsJobsForNodes(ctx, cli); err != nil {
		log.Error(err, "Failed to cleanup artifacts jobs")
		// don't return an error here as it's not critical
	}

	return nil
}

func ensureAirgapArtifactsInCluster(ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation) error {
	log := controllerruntime.LoggerFrom(ctx)

	log.Info("Uploading container images...")

	err := EnsureAirgapArtifactsPlan(ctx, cli, rc, in)
	if err != nil {
		return fmt.Errorf("ensure autopilot plan: %w", err)
	}

	log.Info("Waiting for container images to be uploaded...")

	err = k0s.WaitForAirgapArtifactsAutopilotPlan(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("wait for airgap artifacts autopilot plan: %w", err)
	}

	log.Info("Container images uploaded")
	return nil
}

// EnsureAirgapArtifactsPlan creates the autopilot plan for loading airgap artifacts into containerd.
func EnsureAirgapArtifactsPlan(ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation) error {
	plan, err := getAutopilotAirgapArtifactsPlan(ctx, cli, rc, in)
	if err != nil {
		return fmt.Errorf("get autopilot airgap artifacts plan: %w", err)
	}

	err = kubeutils.EnsureObject(ctx, cli, plan, func(opts *kubeutils.EnsureObjectOptions) {
		opts.ShouldDelete = func(obj client.Object) bool {
			return obj.GetAnnotations()[operatorartifacts.InstallationNameAnnotation] != in.Name
		}
	})
	if err != nil {
		return fmt.Errorf("ensure autopilot plan: %w", err)
	}

	return nil
}

func getAutopilotAirgapArtifactsPlan(ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation) (*v1beta2.Plan, error) {
	var commands []v1beta2.PlanCommand

	// if we are running in an airgap environment all assets are already present in the
	// node and are served by the local-artifact-mirror binary listening on localhost
	// port 50000. we just need to get autopilot to fetch the k0s binary from there.
	command, err := operatorartifacts.CreateAutopilotAirgapPlanCommand(ctx, cli, rc, in)
	if err != nil {
		return nil, fmt.Errorf("create autopilot airgap plan command: %w", err)
	}
	commands = append(commands, *command)

	plan := &v1beta2.Plan{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1beta2.SchemeGroupVersion.String(),
			Kind:       "Plan",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot", // this is a fixed name and should not be changed
			Annotations: map[string]string{
				operatorartifacts.InstallationNameAnnotation: in.Name,
			},
		},
		Spec: v1beta2.PlanSpec{
			Timestamp: "now",
			ID:        uuid.New().String(),
			Commands:  commands,
		},
	}

	return plan, nil
}
