package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	operatorartifacts "github.com/replicatedhq/embedded-cluster/operator/pkg/artifacts"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/metadata"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	pkgartifacts "github.com/replicatedhq/embedded-cluster/pkg-new/artifacts"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	upgradeJobName      = "embedded-cluster-upgrade-%s"
	upgradeJobConfigMap = "upgrade-job-configmap-%s"
)

// CreateUpgradeJob creates a job that upgrades the embedded cluster to the version specified in the installation.
// if the installation is airgapped, the artifacts are copied to the nodes and the autopilot plan is
// created to copy the images to the cluster. A configmap is then created containing the target installation
// spec and the upgrade job is created. The upgrade job will update the cluster version, and then update the operator chart.
func CreateUpgradeJob(
	ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation,
	localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion string,
	previousInstallVersion string,
) error {
	log := controllerruntime.LoggerFrom(ctx)

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, cli)
	if err != nil {
		return fmt.Errorf("get kotsadm namespace: %w", err)
	}

	// check if the job already exists - if it does, we've already rolled out images and can return now
	job := &batchv1.Job{}
	err = cli.Get(ctx, client.ObjectKey{Namespace: kotsadmNamespace, Name: fmt.Sprintf(upgradeJobName, in.Name)}, job)
	if err == nil {
		return nil
	}

	if localArtifactMirrorImage == "" {
		return fmt.Errorf("local artifact mirror image is required")
	}

	err = metadata.CopyVersionMetadataToCluster(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("copy version metadata to cluster: %w", err)
	}

	err = pkgartifacts.DistributeArtifacts(ctx, cli, rc, in, localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion)
	if err != nil {
		return fmt.Errorf("distribute artifacts: %w", err)
	}

	pullPolicy := corev1.PullIfNotPresent
	if in.Spec.AirGap {
		// in airgap installations autopilot will copy the images to the cluster so we can start
		// the new operator.
		pullPolicy = corev1.PullNever
	}

	// create the upgrade job configmap with the target installation spec
	installationData, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("failed to marshal installation spec: %w", err)
	}

	// check if the configmap exists already or if we can just create it
	existingCm := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Namespace: kotsadmNamespace, Name: fmt.Sprintf(upgradeJobConfigMap, in.Name)}, existingCm)
	if err == nil {
		// if the configmap already exists, update it to have the expected data just in case
		existingCm.Data["installation.yaml"] = string(installationData)
		if err = cli.Update(ctx, existingCm); err != nil {
			return fmt.Errorf("failed to update configmap: %w", err)
		}
		return nil
	} else if !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to get configmap: %w", err)
	} else {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: kotsadmNamespace,
				Name:      fmt.Sprintf(upgradeJobConfigMap, in.Name),
			},
			Data: map[string]string{
				"installation.yaml": string(installationData),
			},
		}
		if err = cli.Create(ctx, cm); err != nil {
			return fmt.Errorf("failed to create upgrade job configmap: %w", err)
		}
	}

	operatorImage, err := operatorImageName(ctx, cli, in)
	if err != nil {
		return err
	}

	env := []corev1.EnvVar{
		{
			Name:  "JOB_NAME",
			Value: fmt.Sprintf(upgradeJobName, in.Name),
		},
		{
			Name:  "JOB_NAMESPACE",
			Value: kotsadmNamespace,
		},
	}

	if proxy := rc.ProxySpec(); proxy != nil {
		env = append(env, corev1.EnvVar{
			Name:  "HTTP_PROXY",
			Value: proxy.HTTPProxy,
		})
		env = append(env, corev1.EnvVar{
			Name:  "HTTPS_PROXY",
			Value: proxy.HTTPSProxy,
		})
		env = append(env, corev1.EnvVar{
			Name:  "NO_PROXY",
			Value: proxy.NoProxy,
		})
	}

	// create the upgrade job
	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kotsadmNamespace,
			Name:      fmt.Sprintf(upgradeJobName, in.Name),
			Labels: map[string]string{
				"app.kubernetes.io/instance": "embedded-cluster-upgrade",
				"app.kubernetes.io/name":     "embedded-cluster-upgrade",
			},
			Annotations: map[string]string{
				operatorartifacts.InstallationNameAnnotation: in.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](6), // this is the default
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/instance": "embedded-cluster-upgrade",
						"app.kubernetes.io/name":     "embedded-cluster-upgrade",
					},
				},
				Spec: corev1.PodSpec{
					// The upgrade job can fail if scheduled on a node that hasn't been upgraded yet.
					// Without this affinity, the job might get scheduled on non-upgraded nodes repeatedly,
					// potentially hitting the backoff limit (6) and causing the upgrade to fail.
					// By preferring control plane nodes, which typically get upgraded first in the sequence,
					// we increase the likelihood of successful scheduling while still allowing fallback to
					// other nodes if necessary.
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
								{
									Weight: 100,
									Preference: corev1.NodeSelectorTerm{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "node-role.kubernetes.io/control-plane",
												Operator: corev1.NodeSelectorOpExists,
											},
										},
									},
								},
							},
						},
					},
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: constants.KotsadmServiceAccount,
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: fmt.Sprintf(upgradeJobConfigMap, in.Name),
									},
								},
							},
						},
						{
							Name: "ec-charts-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									// the job gets created by a process inside the kotsadm pod during an upgrade,
									// and kots doesn't (and shouldn't) have permissions to create this directory
									Path: rc.EmbeddedClusterChartsSubDirNoCreate(),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "embedded-cluster-updater",
							Image:           operatorImage,
							ImagePullPolicy: pullPolicy,
							Env:             env,
							Command: []string{
								"/manager",
								"upgrade-job",
								"--installation",
								"/config/installation.yaml",
								"--previous-version",
								previousInstallVersion,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/config",
								},
								{
									Name:      "ec-charts-dir",
									MountPath: rc.EmbeddedClusterChartsSubDirNoCreate(),
									ReadOnly:  true,
								},
							},
						},
					},
				},
			},
		},
	}

	// Add the host CA bundle volume, mount, and env var if it's available in the installation
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

	// Create the job with all configuration in place
	if err = cli.Create(ctx, job); err != nil {
		return fmt.Errorf("failed to create upgrade job: %w", err)
	}

	return nil
}

func ListUpgradeJobs(ctx context.Context, cli client.Client) ([]batchv1.Job, error) {
	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("get kotsadm namespace: %w", err)
	}

	jobs := batchv1.JobList{}
	err = cli.List(ctx, &jobs, client.InNamespace(kotsadmNamespace), client.MatchingLabels{
		"app.kubernetes.io/instance": "embedded-cluster-upgrade",
		"app.kubernetes.io/name":     "embedded-cluster-upgrade",
	})
	if err != nil {
		return nil, fmt.Errorf("list upgrade jobs: %w", err)
	}

	return jobs.Items, nil
}

func operatorImageName(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) (string, error) {
	// determine the image to use for the upgrade job
	meta, err := release.MetadataFor(ctx, in, cli)
	if err != nil {
		return "", fmt.Errorf("failed to get release metadata: %w", err)
	}
	for _, image := range meta.Images {
		if strings.Contains(image, "embedded-cluster-operator-image") {
			// TODO: This will not work in a non-production environment.
			// The domains in the release are used to supply alternative defaults for staging and the dev environment.
			// The GetDomains function will always fall back to production defaults.
			domains := domains.GetDomains(in.Spec.Config, nil)
			image = strings.Replace(image, "proxy.replicated.com", domains.ProxyRegistryDomain, 1)
			return image, nil
		}
	}
	return "", fmt.Errorf("no embedded-cluster-operator image found in release metadata")
}
