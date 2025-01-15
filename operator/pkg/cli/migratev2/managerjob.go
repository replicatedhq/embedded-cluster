package migratev2

import (
	"context"
	"fmt"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/util"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/manager"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _managerInstallJob = batchv1.Job{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "batch/v1",
		Kind:       "Job",
	},
	ObjectMeta: metav1.ObjectMeta{
		Namespace: jobNamespace,
		Name:      "install-v2-manager-DYNAMIC",
		Labels: map[string]string{
			"app": "install-v2-manager",
		},
	},
	Spec: batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyOnFailure,
				Containers: []corev1.Container{
					{
						Name:            "install-v2-manager",
						Image:           "DYNAMIC",
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: &corev1.SecurityContext{
							RunAsUser:  ptr.To(int64(0)),
							Privileged: ptr.To(true),
						},
						Command: []string{
							"/manager", "migrate-v2", "install-manager",
							"--installation", "/ec/installation/installation",
							"--license", "/ec/license/license",
							// "--app-version-label", "DYNAMIC",
						},
						Env: []corev1.EnvVar{
							{
								Name:  "DBUS_SYSTEM_BUS_ADDRESS", // required to run systemctl commands
								Value: "unix:path=/host/run/dbus/system_bus_socket",
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "installation", // required to set runtime config
								MountPath: "/ec/installation",
								ReadOnly:  true,
							},
							{
								Name:      "license", // required to download the manager binary
								MountPath: "/ec/license",
								ReadOnly:  true,
							},
							{
								Name:      "host-run-dbus-system-bus-socket", // required to run systemctl commands
								MountPath: "/host/run/dbus/system_bus_socket",
							},
							{
								Name:      "host-etc-systemd", // required to write systemd unit files
								MountPath: "/etc/systemd",
							},
							{
								Name:      "host-data-dir", // required to materialize files
								MountPath: ecv1beta1.DefaultDataDir,
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
					// {
					// 	Name: "license",
					// 	VolumeSource: corev1.VolumeSource{
					// 		Secret: &corev1.SecretVolumeSource{
					// 			SecretName: "DYNAMIC",
					// 		},
					// 	},
					// },
					{
						Name: "host-run-dbus-system-bus-socket",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/run/dbus/system_bus_socket",
								Type: ptr.To(corev1.HostPathSocket),
							},
						},
					},
					{
						Name: "host-etc-systemd",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/etc/systemd",
								Type: ptr.To(corev1.HostPathDirectory),
							},
						},
					},
					{
						Name: "host-data-dir",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: ecv1beta1.DefaultDataDir,
								Type: ptr.To(corev1.HostPathDirectory),
							},
						},
					},
				},
			},
		},
	},
}

// InstallAndStartManager installs and starts the manager service on the host. This is run in a job
// on all nodes in the cluster.
func InstallAndStartManager(ctx context.Context, licenseID string, licenseEndpoint string, appVersionLabel string) error {
	binPath := runtimeconfig.PathToEmbeddedClusterBinary("manager")

	// TODO: airgap
	err := manager.DownloadBinaryOnline(ctx, binPath, licenseID, licenseEndpoint, appVersionLabel)
	if err != nil {
		return fmt.Errorf("download manager binary: %w", err)
	}

	err = manager.Install(ctx, logrus.Infof, goods.NewMaterializer())
	if err != nil {
		return fmt.Errorf("install manager: %w", err)
	}

	return nil
}

func getManagerInstallJobSpecForNode(
	node corev1.Node, in *ecv1beta1.Installation, operatorImage string,
	licenseSecret string, appVersionLabel string,
) *batchv1.Job {
	job := _managerInstallJob.DeepCopy()

	job.ObjectMeta.Name = getManagerInstallJobName(node)

	job.Spec.Template.Spec.Containers[0].Image = operatorImage
	job.Spec.Template.Spec.Containers[0].Command = append(job.Spec.Template.Spec.Containers[0].Command,
		"--app-version-label", appVersionLabel,
	)

	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "installation",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: in.Name,
				},
			},
		},
	})
	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "license",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: licenseSecret,
			},
		},
	})

	job.Spec.Template.Spec.NodeName = node.Name
	// tolerate all taints
	for _, taint := range node.Spec.Taints {
		job.Spec.Template.Spec.Tolerations = append(job.Spec.Template.Spec.Tolerations, corev1.Toleration{
			Key:      taint.Key,
			Value:    taint.Value,
			Operator: corev1.TolerationOpEqual,
		})
	}

	return job
}

func getManagerInstallJobName(node corev1.Node) string {
	return util.NameWithLengthLimit(jobNamePrefix, node.Name)
}
