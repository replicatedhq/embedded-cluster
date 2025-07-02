package migrate

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	StatusConditionType = "RegistryMigrationStatus"

	dataMigrationPodName = "registry-data-migration"
	serviceAccountName   = "registry-data-migration-serviceaccount"

	// seaweedfsS3SecretName is the name of the secret containing the s3 credentials.
	// This secret name is defined in the values-ha.yaml file in the release metadata.
	seaweedfsS3SecretName = "seaweedfs-s3-rw"
)

// RunDataMigration should be called when transitioning from non-HA to HA airgapped installations.
// This function creates a pod that will scale down the registry deployment then upload the data to s3
// before finally creating a 'migration is complete' secret in the registry namespace. If this secret
// is present, the function will return without reattempting the migration.
func RunDataMigration(ctx context.Context, cli client.Client, kclient kubernetes.Interface, in *ecv1beta1.Installation, image string) (<-chan string, <-chan error, error) {
	progressCh := make(chan string)
	errCh := make(chan error, 1)

	logrus.Debug("Ensuring migration service account")
	err := ensureMigrationServiceAccount(ctx, cli)
	if err != nil {
		return nil, nil, fmt.Errorf("ensure service account: %w", err)
	}

	logrus.Debug("Ensuring s3 access credentials secret")
	err = ensureS3Secret(ctx, cli)
	if err != nil {
		return nil, nil, fmt.Errorf("ensure s3 secret: %w", err)
	}

	logrus.Debug("Ensuring data migration pod")
	_, err = ensureDataMigrationPod(ctx, cli, image)
	if err != nil {
		return nil, nil, fmt.Errorf("ensure pod: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		logrus.Debug("Monitoring pod status")
		defer logrus.Debug("Pod status monitor stopped")
		monitorPodStatus(ctx, cli, errCh)
	}()

	go monitorPodProgress(ctx, kclient, progressCh)

	return progressCh, errCh, nil
}

func ensureDataMigrationPod(ctx context.Context, cli client.Client, image string) (*corev1.Pod, error) {
	pod := newMigrationPod(image)

	err := maybeDeleteDataMigrationPod(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("delete pod: %w", err)
	}

	err = cli.Create(ctx, pod)
	if err != nil {
		return nil, fmt.Errorf("create pod: %w", err)
	}

	return pod, nil
}

func maybeDeleteDataMigrationPod(ctx context.Context, cli client.Client) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: constants.RegistryNamespace,
			Name:      dataMigrationPodName,
		},
	}

	err := cli.Delete(ctx, pod)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("delete pod: %w", err)
		}
		return nil
	}

	err = kubeutils.WaitForPodDeleted(ctx, cli, constants.RegistryNamespace, dataMigrationPodName, &kubeutils.WaitOptions{
		Backoff: &wait.Backoff{
			Steps:    30,
			Duration: 2 * time.Second,
			Factor:   1.0,
			Jitter:   0.1,
		},
	})
	if err != nil {
		return fmt.Errorf("wait for pod deleted: %w", err)
	}

	return nil
}

func monitorPodStatus(ctx context.Context, cli client.Client, errCh chan<- error) {
	defer close(errCh)

	opts := &kubeutils.WaitOptions{
		// 30 minutes
		Backoff: &wait.Backoff{
			Steps:    300,
			Duration: 6 * time.Second,
			Factor:   1,
			Jitter:   0.1,
		},
	}
	pod, err := kubeutils.WaitForPodComplete(ctx, cli, constants.RegistryNamespace, dataMigrationPodName, opts)
	if err != nil {
		errCh <- err
	}
	switch pod.Status.Phase {
	case corev1.PodFailed:
		errCh <- fmt.Errorf("pod failed")
	case corev1.PodSucceeded:
		errCh <- nil
	default:
		errCh <- fmt.Errorf("pod is not succeeded or failed")
	}
}

func monitorPodProgress(ctx context.Context, kclient kubernetes.Interface, progressCh chan<- string) {
	defer close(progressCh)

	for ctx.Err() == nil {
		logrus.Debugf("Streaming registry data migration pod logs")
		podLogs, err := streamPodLogs(ctx, kclient)
		if err != nil {
			if ctx.Err() == nil {
				logrus.Debugf("Failed to stream registry data migration pod logs: %v", err)
			}
		} else {
			err := streamPodProgress(podLogs, progressCh)
			if err != nil {
				logrus.Debugf("Failed to stream registry data migration pod logs: %v", err)
			} else {
				logrus.Debugf("Finished streaming registry data migration pod logs")
			}
		}

		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
}

func streamPodLogs(ctx context.Context, kclient kubernetes.Interface) (io.ReadCloser, error) {
	logOpts := corev1.PodLogOptions{
		Container: "migrate-registry-data",
		Follow:    true,
		TailLines: ptr.To(int64(100)),
	}
	podLogs, err := kclient.CoreV1().Pods(constants.RegistryNamespace).GetLogs(dataMigrationPodName, &logOpts).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("get logs: %w", err)
	}

	return podLogs, nil
}

func streamPodProgress(rc io.ReadCloser, progressCh chan<- string) error {
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Text()
		if isProgressLogLine(line) {
			progressCh <- getProgressFromLogLine(line)
		}
	}
	return scanner.Err()
}

func getProgressArgs(count, total int) []any {
	return []any{
		slog.String("progress", fmt.Sprintf("%d%%", count*100/total)),
		slog.Int("count", count),
		slog.Int("total", total),
	}
}

var progressRegex = regexp.MustCompile(`progress=(\d+%) count=\d+ total=\d+`)

func isProgressLogLine(line string) bool {
	return progressRegex.MatchString(line)
}

func getProgressFromLogLine(line string) string {
	matches := progressRegex.FindStringSubmatch(line)
	if len(matches) != 2 {
		return ""
	}
	return progressRegex.FindStringSubmatch(line)[1]
}

func newMigrationPod(image string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dataMigrationPodName,
			Namespace: constants.RegistryNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		Spec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: serviceAccountName,
			Volumes: []corev1.Volume{
				{
					Name: "registry-data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "registry", // yes it's really just called "registry"
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "migrate-registry-data",
					Image:   image,
					Command: []string{"/manager"},
					Args:    []string{"migrate", "registry-data"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "registry-data",
							MountPath: "/registry",
						},
					},
					EnvFrom: []corev1.EnvFromSource{
						{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: seaweedfsS3SecretName,
								},
							},
						},
					},
				},
			},
		},
	}

	pod.ObjectMeta.Labels = applyRegistryLabels(pod.ObjectMeta.Labels, dataMigrationPodName)

	return pod
}

func ensureMigrationServiceAccount(ctx context.Context, cli client.Client) error {
	newRole := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-data-migration-role",
			Namespace: constants.RegistryNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"create"},
			},
		},
	}
	err := cli.Create(ctx, &newRole)
	if client.IgnoreAlreadyExists(err) != nil {
		return fmt.Errorf("create role: %w", err)
	}

	newServiceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: constants.RegistryNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
	}
	err = cli.Create(ctx, &newServiceAccount)
	if client.IgnoreAlreadyExists(err) != nil {
		return fmt.Errorf("create service account: %w", err)
	}

	newRoleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-data-migration-rolebinding",
			Namespace: constants.RegistryNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     "registry-data-migration-role",
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: constants.RegistryNamespace,
			},
		},
	}

	err = cli.Create(ctx, &newRoleBinding)
	if client.IgnoreAlreadyExists(err) != nil {
		return fmt.Errorf("create role binding: %w", err)
	}

	return nil
}

func ensureS3Secret(ctx context.Context, kcli client.Client) error {
	accessKey, secretKey, err := seaweedfs.GetS3RWCreds(ctx, kcli)
	if err != nil {
		return fmt.Errorf("get seaweedfs s3 rw creds: %w", err)
	}

	var secret corev1.Secret
	err = kcli.Get(ctx, client.ObjectKey{
		Namespace: constants.RegistryNamespace,
		Name:      seaweedfsS3SecretName,
	}, &secret)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("get secret: %w", err)
	} else if err == nil {
		secret.Data = map[string][]byte{
			"s3AccessKey": []byte(accessKey),
			"s3SecretKey": []byte(secretKey),
		}

		if err := kcli.Update(ctx, &secret); err != nil {
			return fmt.Errorf("update secret: %w", err)
		}

		return nil
	}

	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: constants.RegistryNamespace,
			Name:      seaweedfsS3SecretName,
			Labels:    seaweedfs.ApplyLabels(secret.ObjectMeta.Labels, "s3"),
		},
		Data: map[string][]byte{
			"s3AccessKey": []byte(accessKey),
			"s3SecretKey": []byte(secretKey),
		},
	}

	if err := kcli.Create(ctx, &secret); err != nil {
		return fmt.Errorf("create secret: %w", err)
	}

	return nil
}

func applyRegistryLabels(labels map[string]string, component string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	// we do not back this up so we dont add the label app=docker-registry
	labels["app.kubernetes.io/component"] = component
	labels["app.kubernetes.io/part-of"] = "embedded-cluster"
	labels["app.kubernetes.io/managed-by"] = "embedded-cluster-operator"
	return labels
}
