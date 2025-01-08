package migrate

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var job = batchv1.Job{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "embedded-cluster",
		Name:      "install-v2-manager",
	},
	Spec: batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				RestartPolicy:      corev1.RestartPolicyOnFailure,
				ServiceAccountName: "embedded-cluster-operator",
				Containers: []corev1.Container{
					{
						Name:            "install-v2-manager",
						Image:           "DYNAMIC",
						ImagePullPolicy: corev1.PullAlways,
						SecurityContext: &corev1.SecurityContext{
							RunAsUser:  ptr.To(int64(0)),
							Privileged: ptr.To(true),
						},
						Command: []string{
							"/manager", "upgrade", "install-v2-manager",
							"--installation", "/ec/installation",
							"--license-id", "DYNAMIC",
							"--license-endpoint", "DYNAMIC",
							"--version-label", "DYNAMIC",
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
								MountPath: "/ec",
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
					{
						Name: "installation",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "DYNAMIC",
								},
							},
						},
					},
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
