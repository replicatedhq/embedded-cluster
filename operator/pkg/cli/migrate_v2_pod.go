package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/metadata"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	migrateV2PodNamespace = "kotsadm"
	migrateV2PodName      = "migrate-v2"
)

var _migrateV2PodSpec = corev1.Pod{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Pod",
	},
	ObjectMeta: metav1.ObjectMeta{
		Namespace: migrateV2PodNamespace,
		Name:      migrateV2PodName,
		Labels: map[string]string{
			"app": "install-v2-manager",
		},
	},
	Spec: corev1.PodSpec{
		ServiceAccountName: "kotsadm",
		RestartPolicy:      corev1.RestartPolicyNever,
		Containers: []corev1.Container{
			{
				Name:            "install-v2-manager",
				Image:           "DYNAMIC",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{
					"/manager", "migrate-v2",
					"--installation", "/ec/installation/installation",
					// "--migrate-v2-secret", "DYNAMIC",
					// "--app-slug", "DYNAMIC",
					// "--app-version-label", "DYNAMIC",
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "installation", // required to set runtime config
						MountPath: "/ec/installation",
						ReadOnly:  true,
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			// {
			// 	Name: "installation",
			// 	VolumeSource: corev1.VolumeSource{
			// 		ConfigMap: &corev1.ConfigMapVolumeSource{
			// 			LocalObjectReference: corev1.LocalObjectReference{
			// 				Name: "DYNAMIC",
			// 			},
			// 		},
			// 	},
			// },
		},
	},
}

// runMigrateV2PodAndWait runs the v2 migration pod and waits for the pod to finish.
func runMigrateV2PodAndWait(
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

	logf("Ensuring v2 migration pod")
	_, err = ensureMigrateV2Pod(ctx, cli, in, operatorImage, migrationSecret, appSlug, appVersionLabel)
	if err != nil {
		return fmt.Errorf("create pod: %w", err)
	}
	logf("Successfully ensured v2 migration pod")

	logf("Waiting for v2 migration pod to finish")
	err = waitForMigrateV2Pod(ctx, cli)
	if err != nil {
		return fmt.Errorf("wait for pod: %w", err)
	}
	logf("Successfully waited for v2 migration pod to finish")

	// NOTE: the installation config map cannot be deleted because the service account gets deleted
	// during the v2 migration so this pod no longer has permissions.
	//
	// logf("Deleting installation config map")
	// err = deleteInstallationConfigMap(ctx, cli, in)
	// if err != nil {
	// 	return fmt.Errorf("delete installation config map: %w", err)
	// }
	// logf("Successfully deleted installation config map")

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

func ensureMigrateV2Pod(
	ctx context.Context, cli client.Client,
	in *ecv1beta1.Installation, operatorImage string,
	migrationSecret string, appSlug string, appVersionLabel string,
) (string, error) {
	existing, err := getMigrateV2Pod(ctx, cli)
	if err == nil {
		if migrateV2PodHasSucceeded(existing) {
			return existing.Name, nil
		} else if migrateV2PodHasFailed(existing) {
			err := cli.Delete(ctx, &existing)
			if err != nil {
				return "", fmt.Errorf("delete pod %s: %w", existing.Name, err)
			}
		} else {
			// still running
			return existing.Name, nil
		}
	} else if !k8serrors.IsNotFound(err) {
		return "", fmt.Errorf("get pod: %w", err)
	}

	pod := getMigrateV2PodSpec(in, operatorImage, migrationSecret, appSlug, appVersionLabel)
	if err := cli.Create(ctx, pod); err != nil {
		return "", err
	}

	return pod.Name, nil
}

// deleteMigrateV2Pod deletes the v2 migration pod.
func deleteMigrateV2Pod(ctx context.Context, logf LogFunc, cli client.Client) error {
	logf("Deleting v2 migration pod")

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: migrateV2PodNamespace, Name: migrateV2PodName,
		},
	}
	err := cli.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationBackground))
	if err != nil {
		return fmt.Errorf("delete pod: %w", err)
	}

	logf("Successfully deleted v2 migration pod")

	return nil
}

func waitForMigrateV2Pod(ctx context.Context, cli client.Client) error {
	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		pod := corev1.Pod{}
		nsn := apitypes.NamespacedName{Namespace: migrateV2PodNamespace, Name: migrateV2PodName}
		err := cli.Get(ctx, nsn, &pod)
		switch {
		// If we get unauthorized, it means the service account has been deleted by the migration
		// pod and it is likely almost done.
		case k8serrors.IsUnauthorized(err):
			return true, nil
		case err != nil:
			return false, fmt.Errorf("get pod: %w", err)
		case pod.Status.Phase == corev1.PodSucceeded:
			return true, nil
		case pod.Status.Phase == corev1.PodFailed:
			return true, fmt.Errorf("pod failed: %s", pod.Status.Reason)
		default:
			return false, nil
		}
	})
}

func getMigrateV2Pod(ctx context.Context, cli client.Client) (corev1.Pod, error) {
	var pod corev1.Pod
	err := cli.Get(ctx, client.ObjectKey{Namespace: migrateV2PodNamespace, Name: migrateV2PodName}, &pod)
	if err != nil {
		return pod, fmt.Errorf("get pod: %w", err)
	}
	return pod, nil
}

func migrateV2PodHasSucceeded(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded
}

func migrateV2PodHasFailed(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodFailed
}

func getMigrateV2PodSpec(
	in *ecv1beta1.Installation, operatorImage string,
	migrationSecret string, appSlug string, appVersionLabel string,
) *corev1.Pod {
	pod := _migrateV2PodSpec.DeepCopy()

	pod.Spec.Containers[0].Image = operatorImage
	pod.Spec.Containers[0].Command = append(pod.Spec.Containers[0].Command,
		"--migrate-v2-secret", migrationSecret,
		"--app-slug", appSlug,
		"--app-version-label", appVersionLabel,
	)

	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "installation",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: getInstallationConfigMapName(in),
				},
			},
		},
	})

	return pod
}

func ensureInstallationConfigMap(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error {
	copy := in.DeepCopy()
	err := createInstallationConfigMap(ctx, cli, copy)
	if k8serrors.IsAlreadyExists(err) {
		err := updateInstallationConfigMap(ctx, cli, copy)
		if err != nil {
			return fmt.Errorf("update installation config map: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("create installation config map: %w", err)
	}
	return nil
}

func deleteInstallationConfigMap(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: migrateV2PodNamespace,
			Name:      getInstallationConfigMapName(in),
		},
	}
	err := cli.Delete(ctx, &cm)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func createInstallationConfigMap(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error {
	data, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal installation: %w", err)
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: migrateV2PodNamespace,
			Name:      getInstallationConfigMapName(in),
		},
		Data: map[string]string{
			"installation": string(data),
		},
	}
	if err := cli.Create(ctx, cm); err != nil {
		return fmt.Errorf("create configmap: %w", err)
	}

	return nil
}

func updateInstallationConfigMap(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error {
	// find configmap with the same name as the installation
	nsn := apitypes.NamespacedName{Namespace: migrateV2PodNamespace, Name: getInstallationConfigMapName(in)}
	var cm corev1.ConfigMap
	if err := cli.Get(ctx, nsn, &cm); err != nil {
		return fmt.Errorf("get configmap: %w", err)
	}

	// marshal the installation and update the configmap
	data, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal installation: %w", err)
	}
	cm.Data["installation"] = string(data)

	if err := cli.Update(ctx, &cm); err != nil {
		return fmt.Errorf("update configmap: %w", err)
	}
	return nil
}

func getInstallationConfigMapName(in *ecv1beta1.Installation) string {
	return fmt.Sprintf("%s-installation", in.Name)
}
