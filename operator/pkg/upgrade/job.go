package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/metadata"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateUpgradeJob creates a job that upgrades the embedded cluster to the version specified in the installation.
// if the installation is airgapped, the artifacts are copied to the nodes and the autopilot plan is
// created to copy the images to the cluster. A configmap is then created containing the target installation
// spec and the upgrade job is created. The upgrade job will update the cluster version, and then update the operator chart.
func CreateUpgradeJob(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation, localArtifactMirrorImage string) error {
	// check if the job already exists - if it does, we've already rolled out images and can return now
	job := &batchv1.Job{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: "embedded-cluster", Name: fmt.Sprintf(upgradeJobName, in.Name)}, job)
	if err == nil {
		return nil
	}

	pullPolicy := corev1.PullIfNotPresent
	if in.Spec.AirGap {
		// in airgap installations we need to copy the artifacts to the nodes and then autopilot
		// will copy the images to the cluster so we can start the new operator.

		if localArtifactMirrorImage == "" {
			return fmt.Errorf("local artifact mirror image is required for airgap installations")
		}

		err = metadata.CopyVersionMetadataToCluster(ctx, cli, in)
		if err != nil {
			return fmt.Errorf("copy version metadata to cluster: %w", err)
		}

		err = airgapDistributeArtifacts(ctx, cli, in, localArtifactMirrorImage)
		if err != nil {
			return fmt.Errorf("airgap distribute artifacts: %w", err)
		}

		pullPolicy = corev1.PullNever
	}

	// create the installation object so that kotsadm can immediately find it and watch it for the upgrade process
	err = createInstallation(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("apply installation: %w", err)
	}

	// create the upgrade job configmap with the target installation spec
	installationData, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("failed to marshal installation spec: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "embedded-cluster",
			Name:      fmt.Sprintf(upgradeJobConfigMap, in.Name),
		},
		Data: map[string]string{
			"installation.yaml": string(installationData),
		},
	}
	if err = cli.Create(ctx, cm); err != nil {
		return fmt.Errorf("failed to create upgrade job configmap: %w", err)
	}

	operatorImage, err := operatorImageName(ctx, cli, in)
	if err != nil {
		return err
	}

	env := []corev1.EnvVar{
		{
			Name:  "SSL_CERT_DIR",
			Value: "/certs",
		},
	}

	if in.Spec.Proxy != nil {
		env = append(env, corev1.EnvVar{
			Name:  "HTTP_PROXY",
			Value: in.Spec.Proxy.HTTPProxy,
		})
		env = append(env, corev1.EnvVar{
			Name:  "HTTPS_PROXY",
			Value: in.Spec.Proxy.HTTPSProxy,
		})
		env = append(env, corev1.EnvVar{
			Name:  "NO_PROXY",
			Value: in.Spec.Proxy.ProvidedNoProxy,
		})
	}

	// create the upgrade job
	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "embedded-cluster",
			Name:      fmt.Sprintf(upgradeJobName, in.Name),
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: "embedded-cluster-operator",
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
							Name: "private-cas",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "private-cas",
									},
									Optional: ptr.To[bool](true),
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
								"--local-artifact-mirror-image",
								localArtifactMirrorImage,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/config",
								},
								{
									Name:      "private-cas",
									MountPath: "/certs",
								},
							},
						},
					},
				},
			},
		},
	}
	if err = cli.Create(ctx, job); err != nil {
		return fmt.Errorf("failed to create upgrade job: %w", err)
	}

	return nil
}

func operatorImageName(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) (string, error) {
	// determine the image to use for the upgrade job
	meta, err := release.MetadataFor(ctx, in, cli)
	if err != nil {
		return "", fmt.Errorf("failed to get release metadata: %w", err)
	}
	operatorImage := ""
	for _, image := range meta.Images {
		if strings.Contains(image, "embedded-cluster-operator-image") {
			operatorImage = image
			break
		}
	}
	return operatorImage, nil
}
