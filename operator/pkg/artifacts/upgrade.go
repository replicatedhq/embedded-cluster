package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"

	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/release"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ecNamespace = "embedded-cluster"
const copyArtifactsJobPrefix = "copy-artifacts-"

const (
	InstallationNameAnnotation    = "embedded-cluster.replicated.com/installation-name"
	ArtifactsConfigHashAnnotation = "embedded-cluster.replicated.com/artifacts-config-hash"
)

// copyArtifactsJob is a job we create everytime we need to sync files into all nodes.
// This job mounts /var/lib/embedded-cluster from the node and uses binaries that are
// present there. This is not yet a complete version of the job as it misses some env
// variables and a node selector, those are populated during the reconcile cycle.
var copyArtifactsJob = &batchv1.Job{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "batch/v1",
		Kind:       "Job",
	},
	ObjectMeta: metav1.ObjectMeta{
		Namespace: ecNamespace,
	},
	Spec: batchv1.JobSpec{
		BackoffLimit: ptr.To[int32](2),
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				ServiceAccountName: "embedded-cluster-operator",
				Volumes: []corev1.Volume{
					{
						Name: "host",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/var/lib/embedded-cluster",
								Type: ptr.To[corev1.HostPathType]("Directory"),
							},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name: "embedded-cluster-updater",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "host",
								MountPath: "/var/lib/embedded-cluster",
								ReadOnly:  false,
							},
						},
						Command: []string{
							"/bin/sh",
							"-ex",
							"-c",
							"/usr/local/bin/local-artifact-mirror pull binaries $INSTALLATION_DATA\n" +
								"/usr/local/bin/local-artifact-mirror pull images $INSTALLATION_DATA\n" +
								"/usr/local/bin/local-artifact-mirror pull helmcharts $INSTALLATION_DATA\n" +
								"mv /var/lib/embedded-cluster/bin/k0s /var/lib/embedded-cluster/bin/k0s-upgrade\n" +
								"rm /var/lib/embedded-cluster/images/images-amd64-* || true\n" +
								"cd /var/lib/embedded-cluster/images/\n" +
								"mv images-amd64.tar images-amd64-${INSTALLATION}.tar\n" +
								"echo 'done'",
						},
					},
				},
			},
		},
	},
}

// EnsureArtifactsJobForNodes copies the installation artifacts to the nodes in the cluster.
// This is done by creating a job for each node in the cluster, which will pull the
// artifacts from the internal registry.
func EnsureArtifactsJobForNodes(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation, localArtifactMirrorImage string) error {
	if in.Spec.Artifacts == nil {
		return fmt.Errorf("no artifacts location defined")
	}

	var nodes corev1.NodeList
	if err := cli.List(ctx, &nodes); err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	// generate a hash of the current config so we can detect config changes.
	cfghash, err := HashForAirgapConfig(in)
	if err != nil {
		return fmt.Errorf("hash airgap config: %w", err)
	}

	for _, node := range nodes.Items {
		_, err := ensureArtifactsJobForNode(ctx, cli, in, node, localArtifactMirrorImage, cfghash)
		if err != nil {
			return fmt.Errorf("ensure artifacts job for node: %w", err)
		}
	}

	return nil
}

// ListArtifactsJobForNodes list all the artifacts jobs for the nodes in the cluster.
func ListArtifactsJobForNodes(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) (map[string]*batchv1.Job, error) {
	var nodes corev1.NodeList
	if err := cli.List(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	// generate a hash of the current config so we can detect config changes.
	cfghash, err := HashForAirgapConfig(in)
	if err != nil {
		return nil, fmt.Errorf("hash airgap config: %w", err)
	}

	jobs := map[string]*batchv1.Job{}

	for _, node := range nodes.Items {
		nsn := types.NamespacedName{
			Name:      util.NameWithLengthLimit(copyArtifactsJobPrefix, node.Name),
			Namespace: ecNamespace,
		}

		job := &batchv1.Job{}
		err := cli.Get(ctx, nsn, job)
		if err == nil {
			// we need to check if the job is for the given installation otherwise we delete
			// it. we also need to check if the configuration has changed. this will trigger
			// a new reconcile cycle.
			annotations := job.GetAnnotations()
			oldjob := annotations[InstallationNameAnnotation] != in.Name
			newcfg := annotations[ArtifactsConfigHashAnnotation] != cfghash
			if !oldjob && !newcfg {
				jobs[node.Name] = job
				continue
			}
		} else if !k8serrors.IsNotFound(err) {
			return nil, fmt.Errorf("get job: %w", err)
		}

		jobs[node.Name] = nil
	}

	return jobs, nil
}

// HashForAirgapConfig generates a hash for the airgap configuration. We can use this to detect config changes between
// different reconcile cycles.
func HashForAirgapConfig(in *clusterv1beta1.Installation) (string, error) {
	data, err := json.Marshal(in.Spec.Artifacts)
	if err != nil {
		return "", fmt.Errorf("failed to marshal artifacts location: %w", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	return hash[:10], nil
}

func ensureArtifactsJobForNode(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation, node corev1.Node, localArtifactMirrorImage, cfghash string) (*batchv1.Job, error) {
	job, err := getArtifactJobForNode(ctx, cli, in, node, localArtifactMirrorImage)
	if err != nil {
		return nil, fmt.Errorf("get job for node: %w", err)
	}

	err = k8sutil.EnsureObject(ctx, cli, job, func(opts *k8sutil.EnsureObjectOptions) {
		opts.DeleteOptions = append(opts.DeleteOptions, client.PropagationPolicy(metav1.DeletePropagationForeground))
		opts.ShouldDelete = func(obj client.Object) bool {
			// we need to check if the job is for the given installation otherwise we delete
			// it. we also need to check if the configuration has changed. this will trigger
			// a new reconcile cycle.
			annotations := obj.GetAnnotations()
			oldjob := annotations[InstallationNameAnnotation] != in.Name
			newcfg := annotations[ArtifactsConfigHashAnnotation] != cfghash
			return oldjob || newcfg
		}
	})
	if err != nil {
		return nil, fmt.Errorf("ensure object: %w", err)
	}

	return job, nil
}

func getArtifactJobForNode(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation, node corev1.Node, localArtifactMirrorImage string) (*batchv1.Job, error) {
	hash, err := HashForAirgapConfig(in)
	if err != nil {
		return nil, fmt.Errorf("failed to hash airgap config: %w", err)
	}

	inData, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal installation: %w", err)
	}
	inDataEncoded := base64.StdEncoding.EncodeToString(inData)

	job := copyArtifactsJob.DeepCopy()
	job.ObjectMeta.Name = util.NameWithLengthLimit(copyArtifactsJobPrefix, node.Name)
	job.ObjectMeta.Labels = applyECOperatorLabels(job.ObjectMeta.Labels, "upgrader")
	job.ObjectMeta.Annotations = applyArtifactsJobAnnotations(job.GetAnnotations(), in, hash)
	job.Spec.Template.Spec.NodeName = node.Name
	job.Spec.Template.Spec.Containers[0].Env = append(
		job.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "INSTALLATION", Value: in.Name},
		corev1.EnvVar{Name: "INSTALLATION_DATA", Value: inDataEncoded},
	)

	job.Spec.Template.Spec.Containers[0].Image = localArtifactMirrorImage
	job.Spec.Template.Spec.ImagePullSecrets = append(job.Spec.Template.Spec.ImagePullSecrets, GetRegistryImagePullSecret())

	if in.GetUID() != "" {
		err = ctrl.SetControllerReference(in, job, cli.Scheme())
		if err != nil {
			return nil, fmt.Errorf("failed to set controller reference: %w", err)
		}
	}

	return job, nil
}

// CreateAutopilotAirgapPlanCommand creates the plan to execute an aigrap upgrade in all nodes. The
// return of this function is meant to be used as part of an autopilot plan.
func CreateAutopilotAirgapPlanCommand(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) (*autopilotv1beta2.PlanCommand, error) {
	meta, err := release.MetadataFor(ctx, in, cli)
	if err != nil {
		return nil, fmt.Errorf("failed to get release metadata: %w", err)
	}

	var nodes corev1.NodeList
	if err := cli.List(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var allNodes []string
	for _, node := range nodes.Items {
		allNodes = append(allNodes, node.Name)
	}

	imageURL := fmt.Sprintf("http://127.0.0.1:50000/images/images-amd64-%s.tar", in.Name)

	return &autopilotv1beta2.PlanCommand{
		AirgapUpdate: &autopilotv1beta2.PlanCommandAirgapUpdate{
			Version: meta.Versions["Kubernetes"],
			Platforms: map[string]autopilotv1beta2.PlanResourceURL{
				"linux-amd64": {
					URL: imageURL,
				},
			},
			Workers: autopilotv1beta2.PlanCommandTarget{
				Discovery: autopilotv1beta2.PlanCommandTargetDiscovery{
					Static: &autopilotv1beta2.PlanCommandTargetDiscoveryStatic{
						Nodes: allNodes,
					},
				},
			},
		},
	}, nil
}

func applyArtifactsJobAnnotations(annotations map[string]string, in *clusterv1beta1.Installation, hash string) map[string]string {
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[InstallationNameAnnotation] = in.Name
	annotations[ArtifactsConfigHashAnnotation] = hash
	return annotations
}
