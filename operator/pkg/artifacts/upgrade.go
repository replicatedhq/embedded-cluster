package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/util"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"go.uber.org/multierr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ecNamespace = "embedded-cluster"
const copyArtifactsJobPrefix = "copy-artifacts-"
const licenseIDSecretName = "embedded-cluster-license-id"

const (
	// InstallationNameAnnotation is the annotation we keep in the autopilot plan so we can
	// map 1 to 1 one installation and one plan.
	InstallationNameAnnotation    = "embedded-cluster.replicated.com/installation-name"
	ArtifactsConfigHashAnnotation = "embedded-cluster.replicated.com/artifacts-config-hash"
)

// copyArtifactsJob is a job we create everytime we need to sync files into all nodes. This job
// mounts the data directory from the node and uses binaries that are present there. This is not
// yet a complete version of the job as it misses some env variables and a node selector, those are
// populated during the reconcile cycle.
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
								Path: clusterv1beta1.DefaultDataDir,
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
								MountPath: "/embedded-cluster",
								ReadOnly:  false,
							},
						},
					},
				},
			},
		},
	},
}

var copyArtifactsJobCommandOnline = []string{
	"/bin/sh",
	"-ex",
	"-c",
	"/usr/local/bin/local-artifact-mirror pull binaries --data-dir /embedded-cluster " +
		"--app-slug $APP_SLUG --channel-id $CHANNEL_ID --app-version $APP_VERSION " +
		"$INSTALLATION_DATA; \n" +
		"sleep 10; \n" + // wait for LAM to restart so k0s can pull from it. LAM restarts when it detects an EC binary update.
		"echo 'done'",
}

var copyArtifactsJobCommandAirgap = []string{
	"/bin/sh",
	"-ex",
	"-c",
	"/usr/local/bin/local-artifact-mirror pull binaries --data-dir /embedded-cluster $INSTALLATION_DATA; \n" +
		"/usr/local/bin/local-artifact-mirror pull images --data-dir /embedded-cluster $INSTALLATION_DATA; \n" +
		"/usr/local/bin/local-artifact-mirror pull helmcharts --data-dir /embedded-cluster $INSTALLATION_DATA; \n" +
		"mv /embedded-cluster/bin/k0s /embedded-cluster/bin/k0s-upgrade; \n" +
		"rm /embedded-cluster/images/images-amd64-* || true; \n" +
		"sleep 10; \n" + // wait for LAM to restart so k0s can pull from it. LAM restarts when it detects an EC binary update.
		"echo 'done'",
}

// EnsureArtifactsJobForNodes copies the installation artifacts to the nodes in the cluster.
// This is done by creating a job for each node in the cluster, which will pull the
// artifacts from the internal registry.
func EnsureArtifactsJobForNodes(
	ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig,
	in *clusterv1beta1.Installation,
	localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion string,
) error {
	if in.Spec.AirGap && in.Spec.Artifacts == nil {
		return fmt.Errorf("no artifacts location defined")
	}

	// Ensure license ID secret exists
	if err := ensureLicenseIDSecret(ctx, cli, licenseID); err != nil {
		return fmt.Errorf("ensure license ID secret: %w", err)
	}

	var nodes corev1.NodeList
	if err := cli.List(ctx, &nodes); err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	// generate a hash of the current config so we can detect config changes.
	cfghash, err := hashForAirgapConfig(in)
	if err != nil {
		return fmt.Errorf("hash airgap config: %w", err)
	}

	for _, node := range nodes.Items {
		_, err := ensureArtifactsJobForNode(
			ctx, cli, rc, in, node, localArtifactMirrorImage, appSlug, channelID, appVersion, cfghash,
		)
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
	cfghash, err := hashForAirgapConfig(in)
	if err != nil {
		return nil, fmt.Errorf("hash airgap config: %w", err)
	}

	jobs := map[string]*batchv1.Job{}

	for _, node := range nodes.Items {
		nsn := client.ObjectKey{
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

// CleanupArtifactsJobsForNodes deletes the jobs and the secret created by
// EnsureArtifactsJobForNodes.
func CleanupArtifactsJobsForNodes(ctx context.Context, cli client.Client) (finalErr error) {
	var nodes corev1.NodeList
	if err := cli.List(ctx, &nodes); err != nil {
		finalErr = multierr.Append(finalErr, fmt.Errorf("list nodes: %w", err))
	}

	for _, node := range nodes.Items {
		err := deleteArtifactsJobForNode(ctx, cli, node)
		if err != nil {
			finalErr = multierr.Append(finalErr, fmt.Errorf("delete job for node %s: %w", node.Name, err))
		}
	}
	err := deleteLicenseIDSecret(ctx, cli)
	if err != nil {
		finalErr = multierr.Append(finalErr, fmt.Errorf("delete license ID secret: %w", err))
	}

	return finalErr
}

// hashForAirgapConfig generates a hash for the airgap configuration. We can use this to detect config changes between
// different reconcile cycles.
func hashForAirgapConfig(in *clusterv1beta1.Installation) (string, error) {
	if !in.Spec.AirGap {
		return "", nil
	}

	data, err := json.Marshal(in.Spec.Artifacts)
	if err != nil {
		return "", fmt.Errorf("failed to marshal artifacts location: %w", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	return hash[:10], nil
}

func ensureArtifactsJobForNode(
	ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig, in *clusterv1beta1.Installation,
	node corev1.Node,
	localArtifactMirrorImage, appSlug, channelID, appVersion string,
	cfghash string,
) (*batchv1.Job, error) {
	job, err := getArtifactJobForNode(ctx, cli, rc, in, node, localArtifactMirrorImage, appSlug, channelID, appVersion)
	if err != nil {
		return nil, fmt.Errorf("get job for node: %w", err)
	}

	err = kubeutils.EnsureObject(ctx, cli, job, func(opts *kubeutils.EnsureObjectOptions) {
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

func getArtifactJobForNode(
	ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig, in *clusterv1beta1.Installation,
	node corev1.Node,
	localArtifactMirrorImage, appSlug, channelID, appVersion string,
) (*batchv1.Job, error) {
	hash, err := hashForAirgapConfig(in)
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
	job.Spec.Template.Spec.Volumes[0].VolumeSource.HostPath.Path = rc.EmbeddedClusterHomeDirectory()
	if in.Spec.AirGap {
		job.Spec.Template.Spec.Containers[0].Command = copyArtifactsJobCommandAirgap
	} else {
		job.Spec.Template.Spec.Containers[0].Command = copyArtifactsJobCommandOnline
	}
	job.Spec.Template.Spec.Containers[0].Env = append(
		job.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "INSTALLATION", Value: in.Name},
		corev1.EnvVar{Name: "INSTALLATION_DATA", Value: inDataEncoded},
		corev1.EnvVar{Name: "LOCAL_ARTIFACT_MIRROR_LICENSE_ID", ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: licenseIDSecretName},
				Key:                  "LICENSE_ID",
			},
		}},
		corev1.EnvVar{Name: "APP_SLUG", Value: appSlug},
		corev1.EnvVar{Name: "CHANNEL_ID", Value: channelID},
		corev1.EnvVar{Name: "APP_VERSION", Value: appVersion},
	)

	// Add proxy environment variables if proxy is configured
	if proxy := rc.ProxySpec(); proxy != nil {
		job.Spec.Template.Spec.Containers[0].Env = append(
			job.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{Name: "HTTP_PROXY", Value: proxy.HTTPProxy},
			corev1.EnvVar{Name: "HTTPS_PROXY", Value: proxy.HTTPSProxy},
			corev1.EnvVar{Name: "NO_PROXY", Value: proxy.NoProxy},
		)
	}

	// Add the host CA bundle volume, mount, and env var if it's available in the installation
	log := ctrl.LoggerFrom(ctx)
	hostCABundlePath := ""
	if in.Spec.RuntimeConfig != nil {
		hostCABundlePath = in.Spec.RuntimeConfig.HostCABundlePath
	}

	if hostCABundlePath != "" {
		log.Info("Using host CA bundle from installation", "path", hostCABundlePath)

		// Add the CA bundle volume
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "host-ca-bundle",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: hostCABundlePath,
					Type: ptr.To(corev1.HostPathFileOrCreate),
				},
			},
		})

		// Add the CA bundle mount
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			job.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "host-ca-bundle",
				MountPath: "/certs/ca-certificates.crt",
			},
		)

		// Add the SSL_CERT_DIR environment variable
		job.Spec.Template.Spec.Containers[0].Env = append(
			job.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  "SSL_CERT_DIR",
				Value: "/certs",
			},
		)
	} else {
		log.Info("No host CA bundle path found in installation, no CA bundle will be used")
	}

	if !in.Spec.AirGap && in.Spec.Config != nil && in.Spec.Config.Domains.ProxyRegistryDomain != "" {
		localArtifactMirrorImage = strings.Replace(
			localArtifactMirrorImage,
			"proxy.replicated.com", in.Spec.Config.Domains.ProxyRegistryDomain, 1,
		)
	}
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

// deleteArtifactsJobForNode deletes the artifacts job for a given node.
func deleteArtifactsJobForNode(ctx context.Context, cli client.Client, node corev1.Node) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.NameWithLengthLimit(copyArtifactsJobPrefix, node.Name),
			Namespace: ecNamespace,
		},
	}
	return client.IgnoreNotFound(cli.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)))
}

// CreateAutopilotAirgapPlanCommand creates the plan to execute an aigrap upgrade in all nodes. The
// return of this function is meant to be used as part of an autopilot plan.
func CreateAutopilotAirgapPlanCommand(ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig, in *clusterv1beta1.Installation) (*autopilotv1beta2.PlanCommand, error) {
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

	imageURL := fmt.Sprintf(
		"http://127.0.0.1:%d/images/ec-images-amd64.tar",
		rc.LocalArtifactMirrorPort(),
	)

	return &autopilotv1beta2.PlanCommand{
		AirgapUpdate: &autopilotv1beta2.PlanCommandAirgapUpdate{
			Version: meta.Versions["Kubernetes"],
			Platforms: map[string]autopilotv1beta2.PlanResourceURL{
				fmt.Sprintf("%s-%s", helpers.ClusterOS(), helpers.ClusterArch()): {
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

// ensureLicenseIDSecret deletes the secret if it exists and creates a new one
func ensureLicenseIDSecret(ctx context.Context, cli client.Client, licenseID string) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      licenseIDSecretName,
			Namespace: ecNamespace,
			Labels:    applyECOperatorLabels(nil, "license"),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"LICENSE_ID": []byte(licenseID),
		},
	}
	// delete the secret if it exists
	err := client.IgnoreNotFound(cli.Delete(ctx, secret))
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}

	return cli.Create(ctx, secret)
}

// deleteLicenseIDSecret deletes the license ID secret
func deleteLicenseIDSecret(ctx context.Context, cli client.Client) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      licenseIDSecretName,
			Namespace: ecNamespace,
		},
	}
	return client.IgnoreNotFound(cli.Delete(ctx, secret))
}
