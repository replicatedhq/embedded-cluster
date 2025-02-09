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
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	StatusConditionType = "RegistryMigrationStatus"

	dataMigrationCompleteSecretName = "registry-data-migration-complete"
	dataMigrationJobName            = "registry-data-migration"
	serviceAccountName              = "registry-data-migration-serviceaccount"

	// seaweedfsS3SecretName is the name of the secret containing the s3 credentials.
	// This secret name is defined in the chart in the release metadata.
	seaweedfsS3SecretName = "seaweedfs-s3-rw"
)

// RunDataMigrationJob should be called when transitioning from non-HA to HA airgapped
// installations. This function creates a job that will scale down the registry deployment then
// upload the data to s3 before finally creating a 'migration is complete' secret in the registry
// namespace. If this secret is present, the function will return without reattempting the
// migration.
func RunDataMigrationJob(ctx context.Context, cli client.Client, kclient kubernetes.Interface, in *ecv1beta1.Installation, image string) (<-chan string, <-chan error, error) {
	progressCh := make(chan string)
	errCh := make(chan error, 1)

	// TODO: this should be an argument
	clientset, err := kubeutils.GetClientset()
	if err != nil {
		return nil, nil, fmt.Errorf("get kubernetes clientset: %w", err)
	}

	hasMigrated, err := hasRegistryMigrated(ctx, cli)
	if err != nil {
		return nil, nil, fmt.Errorf("check if registry has migrated before running migration: %w", err)
	} else if hasMigrated {
		close(progressCh)
		errCh <- nil
		return progressCh, errCh, nil
	}

	// TODO: should we check seaweedfs health?

	logrus.Debug("Ensuring migration service account")
	err = ensureMigrationServiceAccount(ctx, cli)
	if err != nil {
		return nil, nil, fmt.Errorf("ensure service account: %w", err)
	}

	logrus.Debug("Ensuring s3 access credentials secret")
	err = ensureS3Secret(ctx, cli)
	if err != nil {
		return nil, nil, fmt.Errorf("ensure s3 secret: %w", err)
	}

	logrus.Debug("Ensuring data migration job")
	_, err = ensureDataMigrationJob(ctx, cli, image)
	if err != nil {
		return nil, nil, fmt.Errorf("ensure job: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		logrus.Debug("Monitoring job status")
		defer logrus.Debug("Job status monitor stopped")
		monitorJobStatus(ctx, cli, errCh)
	}()

	go monitorJobProgress(ctx, cli, clientset, progressCh)

	return progressCh, errCh, nil
}

func ensureDataMigrationJob(ctx context.Context, cli client.Client, image string) (*batchv1.Job, error) {
	job := newMigrationJob(image)

	err := kubeutils.EnsureObject(ctx, cli, job, func(opts *kubeutils.EnsureObjectOptions) {
		opts.DeleteOptions = append(opts.DeleteOptions, client.PropagationPolicy(metav1.DeletePropagationForeground))
		opts.ShouldDelete = func(obj client.Object) bool {
			exceedsBackoffLimit := job.Status.Failed > *job.Spec.BackoffLimit
			return exceedsBackoffLimit
		}
	})
	if err != nil {
		return nil, fmt.Errorf("ensure object: %w", err)
	}
	return job, nil
}

func monitorJobStatus(ctx context.Context, cli client.Client, errCh chan<- error) {
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
	err := kubeutils.WaitForJob(ctx, cli, runtimeconfig.RegistryNamespace, dataMigrationJobName, 1, opts)
	errCh <- err
}

func monitorJobProgress(ctx context.Context, cli client.Client, kclient kubernetes.Interface, progressCh chan<- string) {
	defer close(progressCh)

	for ctx.Err() == nil {
		logrus.Debugf("Streaming registry data migration pod logs")
		podLogs, err := streamJobLogs(ctx, cli, kclient)
		if err != nil {
			if ctx.Err() == nil {
				if ok, err := isJobRetrying(ctx, cli); err != nil {
					logrus.Debugf("Failed to check if data migration job is retrying: %v", err)
				} else if ok {
					progressCh <- "failed, retrying with backoff..."
				} else {
					logrus.Debugf("Failed to stream registry data migration pod logs: %v", err)
				}
			}
		} else {
			err := streamJobProgress(podLogs, progressCh)
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

func streamJobLogs(ctx context.Context, cli client.Client, kclient kubernetes.Interface) (io.ReadCloser, error) {
	podList := &corev1.PodList{}
	err := cli.List(ctx, podList, client.InNamespace(runtimeconfig.RegistryNamespace), client.MatchingLabels{"job-name": dataMigrationJobName})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	var podName string
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			podName = pod.Name
			break
		}
	}
	if podName == "" {
		return nil, fmt.Errorf("no running pods found")
	}

	logOpts := corev1.PodLogOptions{
		Container: "migrate-registry-data",
		Follow:    true,
		TailLines: ptr.To(int64(100)),
	}
	podLogs, err := kclient.CoreV1().Pods(runtimeconfig.RegistryNamespace).GetLogs(podName, &logOpts).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("get logs: %w", err)
	}

	return podLogs, nil
}

func isJobRetrying(ctx context.Context, cli client.Client) (bool, error) {
	job := &batchv1.Job{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: runtimeconfig.RegistryNamespace, Name: dataMigrationJobName}, job)
	if err != nil {
		return false, fmt.Errorf("get job: %w", err)
	}
	if job.Status.Succeeded >= 1 {
		// job is successful
		return false, nil
	}
	if job.Status.Failed == 0 {
		// job has not yet tried
		return false, nil
	}
	exceedsBackoffLimit := job.Status.Failed > *job.Spec.BackoffLimit
	return !exceedsBackoffLimit, nil
}

func streamJobProgress(rc io.ReadCloser, progressCh chan<- string) error {
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

func newMigrationJob(image string) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dataMigrationJobName,
			Namespace: runtimeconfig.RegistryNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](6), // this is the default
			Template: corev1.PodTemplateSpec{
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
			},
		},
	}

	job.ObjectMeta.Labels = applyRegistryLabels(job.ObjectMeta.Labels, dataMigrationJobName)

	return job
}

func ensureMigrationServiceAccount(ctx context.Context, cli client.Client) error {
	newRole := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-data-migration-role",
			Namespace: runtimeconfig.RegistryNamespace,
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
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create role: %w", err)
	}

	newServiceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: runtimeconfig.RegistryNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
	}
	err = cli.Create(ctx, &newServiceAccount)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create service account: %w", err)
	}

	newRoleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-data-migration-rolebinding",
			Namespace: runtimeconfig.RegistryNamespace,
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
				Namespace: runtimeconfig.RegistryNamespace,
			},
		},
	}

	err = cli.Create(ctx, &newRoleBinding)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create role binding: %w", err)
	}

	return nil
}

func ensureS3Secret(ctx context.Context, kcli client.Client) error {
	accessKey, secretKey, err := seaweedfs.GetS3RWCreds(ctx, kcli)
	if err != nil {
		return fmt.Errorf("get seaweedfs s3 rw creds: %w", err)
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: runtimeconfig.RegistryNamespace, Name: seaweedfsS3SecretName},
		Data: map[string][]byte{
			"s3AccessKey": []byte(accessKey),
			"s3SecretKey": []byte(secretKey),
		},
	}

	obj.ObjectMeta.Labels = seaweedfs.ApplyLabels(obj.ObjectMeta.Labels, "s3")

	if err := kcli.Create(ctx, obj); err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create secret: %w", err)
	}
	return nil
}

// hasRegistryMigrated checks if the registry data has been migrated by looking for the 'migration complete' secret in the registry namespace
func hasRegistryMigrated(ctx context.Context, cli client.Client) (bool, error) {
	sec := corev1.Secret{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: runtimeconfig.RegistryNamespace, Name: dataMigrationCompleteSecretName}, &sec)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get registry migration secret: %w", err)
	}

	return true, nil
}

func maybeDeleteRegistryJob(ctx context.Context, cli client.Client) error {
	err := cli.Delete(ctx, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: runtimeconfig.RegistryNamespace,
			Name:      dataMigrationJobName,
		},
	})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete registry migration job: %w", err)
	}
	return nil
}

func applyRegistryLabels(labels map[string]string, component string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	// TODO: why would we want to back this up?
	// labels["app"] = "docker-registry" // this is the backup/restore label for the registry
	labels["app.kubernetes.io/component"] = component
	labels["app.kubernetes.io/part-of"] = "embedded-cluster"
	labels["app.kubernetes.io/managed-by"] = "embedded-cluster-operator"
	return labels
}
