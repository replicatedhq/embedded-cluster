package controllers

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var nodeDeleteTokenJobTemplate = &batchv1.Job{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: ecNamespace,
		Labels:    map[string]string{},
	},
	Spec: batchv1.JobSpec{
		BackoffLimit: ptr.To(int32(2)),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app.kubernetes.io/component": "node-delete-token-delivery",
				},
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "embedded-cluster-operator",
				Volumes: []corev1.Volume{
					{
						Name: "host",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/var/lib/embedded-cluster",
								Type: ptr.To(corev1.HostPathDirectory),
							},
						},
					},
					{
						Name: "k0s",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: runtimeconfig.K0sBinaryPath,
								Type: ptr.To(corev1.HostPathFile),
							},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:  "node-delete-token-delivery",
						Image: "busybox:latest",
						Command: []string{
							"/bin/sh",
							"-c",
							"",
						},
						Env: []corev1.EnvVar{
							{
								Name:  "KUBECONFIG",
								Value: "",
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "host",
								MountPath: "/embedded-cluster",
								ReadOnly:  false,
							},
							{
								Name:      "k0s",
								MountPath: runtimeconfig.K0sBinaryPath,
								ReadOnly:  true,
							},
						},
					},
				},
			},
		},
	},
}

// ReconcileNodeDeleteRBAC ensures per-node RBAC exists for all current worker nodes, creates
// token delivery jobs for any worker node that does not yet have one, and cleans up RBAC for nodes
// that have been removed. Jobs persist after completion (no TTLSecondsAfterFinished) so this check
// avoids recreating jobs on every reconcile.
func (r *InstallationReconciler) ReconcileNodeDeleteRBAC(ctx context.Context, events *NodeEventsBatch) error {
	log := log.FromContext(ctx)

	for _, event := range events.NodesRemoved {
		log.Info("Cleaning up node-delete RBAC for removed node", "node", event.NodeName)
		if err := kubeutils.DeleteNodeDeleteRBAC(ctx, r.Client, ecNamespace, event.NodeName); err != nil {
			log.Error(err, "Failed to cleanup node-delete RBAC", "node", event.NodeName)
		}
	}

	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	var workerCount int
	for _, node := range nodes.Items {
		if _, isController := node.Labels["node-role.kubernetes.io/control-plane"]; isController {
			continue
		}
		workerCount++
		if err := kubeutils.EnsureNodeDeleteRBAC(ctx, r.Client, ecNamespace, node.Name); err != nil {
			return fmt.Errorf("ensure node-delete RBAC for %s: %w", node.Name, err)
		}
		jobName := kubeutils.NodeDeleteTokenJobName(node.Name)
		var existingJob batchv1.Job
		if err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: ecNamespace}, &existingJob); err != nil {
			if k8serrors.IsNotFound(err) {
				log.Info("Creating node-delete token delivery job", "node", node.Name, "job", jobName)
				if err := r.createNodeDeleteTokenDeliveryJob(ctx, node.Name); err != nil {
					return fmt.Errorf("create node-delete token delivery job for %s: %w", node.Name, err)
				}
				continue
			}
			return fmt.Errorf("check node-delete token delivery job for %s: %w", node.Name, err)
		}

		// Job exists — check its terminal status
		var isComplete, isFailed bool
		for _, cond := range existingJob.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
				isComplete = true
				break
			}
			if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
				isFailed = true
				break
			}
		}
		if isComplete {
			log.Info("Node-delete token delivery job already completed", "node", node.Name, "job", jobName)
			continue
		}
		if isFailed {
			log.Info("Recreating failed node-delete token delivery job", "node", node.Name, "job", jobName)
			if err := r.Delete(ctx, &existingJob); err != nil {
				return fmt.Errorf("delete failed node-delete token delivery job for %s: %w", node.Name, err)
			}
			if err := r.createNodeDeleteTokenDeliveryJob(ctx, node.Name); err != nil {
				return fmt.Errorf("create node-delete token delivery job for %s: %w", node.Name, err)
			}
			continue
		}
		log.Info("Node-delete token delivery job still active", "node", node.Name, "job", jobName)
	}

	log.Info("Reconciled node-delete RBAC", "workers", workerCount, "removed", len(events.NodesRemoved))
	return nil
}

func (r *InstallationReconciler) createNodeDeleteTokenDeliveryJob(ctx context.Context, nodeName string) error {
	labels := map[string]string{
		"embedded-cluster/node-name": nodeName,
	}

	job := nodeDeleteTokenJobTemplate.DeepCopy()
	job.Name = kubeutils.NodeDeleteTokenJobName(nodeName)
	for k, v := range labels {
		job.Labels[k] = v
		job.Spec.Template.Labels[k] = v
	}
	job.Spec.Template.Spec.NodeName = nodeName
	job.Spec.Template.Spec.Volumes[0].HostPath.Path = r.RuntimeConfig.EmbeddedClusterHomeDirectory()

	secretName := kubeutils.NodeDeleteSecretName(nodeName)
	tokenFileName := kubeutils.NodeDeleteTokenFileName

	job.Spec.Template.Spec.Containers[0].Command[2] = fmt.Sprintf(
		`for i in $(seq 1 30); do
  TOKEN_B64=$(/embedded-cluster/bin/kubectl get secret %s -n %s -o jsonpath='{.data.token}' 2>/dev/null || true)
  if [ -n "$TOKEN_B64" ]; then
    printf "%%s" "$TOKEN_B64" | base64 -d > /embedded-cluster/%s
    chmod 600 /embedded-cluster/%s
    exit 0
  fi
  sleep 1
done
echo "Timeout waiting for secret token"
exit 1`,
		secretName,
		ecNamespace,
		tokenFileName,
		tokenFileName,
	)

	// overrides the job image if the environment says so.
	if img := os.Getenv("EMBEDDEDCLUSTER_UTILS_IMAGE"); img != "" {
		job.Spec.Template.Spec.Containers[0].Image = img
	}

	if err := r.Create(ctx, job); err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}
