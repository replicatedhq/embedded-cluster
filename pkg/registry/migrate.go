package registry

import (
	"context"
	"fmt"
	"os"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/k8sutil"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const RegistryDataMigrationCompleteSecretName = "registry-data-migration-complete"
const registryDataMigrationJobName = "registry-data-migration"

const RegistryMigrationStatusConditionType = "RegistryMigrationStatus"
const RegistryMigrationServiceAccountName = "registry-data-migration-serviceaccount"

// MigrateRegistryData should be called when transitioning from non-HA to HA airgapped installations
// this function creates a job that will scale down the registry deployment then upload the data to s3
// before finally creating a 'migration is complete' secret in the registry namespace
// if this secret is present, the function will return without reattempting the migration
func MigrateRegistryData(ctx context.Context, in *clusterv1beta1.Installation, cli client.Client) error {
	migrationStatus := k8sutil.CheckConditionStatus(in.Status, RegistryMigrationStatusConditionType)
	if migrationStatus == metav1.ConditionTrue {
		return nil
	}

	hasMigrated, err := HasRegistryMigrated(ctx, cli)
	if err != nil {
		return fmt.Errorf("check if registry has migrated before running migration: %w", err)
	}
	if hasMigrated {
		in.Status.SetCondition(metav1.Condition{
			Type:               RegistryMigrationStatusConditionType,
			Status:             metav1.ConditionTrue,
			Reason:             "MigrationJobCompleted",
			ObservedGeneration: in.Generation,
		})
		return nil
	}

	// if seaweed hasn't finished deploying yet, return without attempting the migration
	seaweedDeployed, err := k8sutil.GetChartHealth(ctx, cli, "seaweedfs")
	if err != nil {
		return fmt.Errorf("check seaweed chart health: %w", err)
	}
	if !seaweedDeployed {
		in.Status.SetCondition(metav1.Condition{
			Type:               RegistryMigrationStatusConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             "SeaweedChartNotDeployed",
			ObservedGeneration: in.Generation,
		})
		return nil
	}

	// check if the migration is already in progress
	// if it is, return without reattempting the migration
	migrationJob := batchv1.Job{}
	err = cli.Get(ctx, client.ObjectKey{Namespace: registryNamespace, Name: registryDataMigrationJobName}, &migrationJob)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("get migration job: %w", err)
		}
	} else {
		if migrationJob.Status.Active > 0 {
			in.Status.SetCondition(metav1.Condition{
				Type:               RegistryMigrationStatusConditionType,
				Status:             metav1.ConditionFalse,
				Reason:             "MigrationJobInProgress",
				ObservedGeneration: in.Generation,
			})
			return nil
		}
		if migrationJob.Status.Failed > 0 {
			in.Status.SetCondition(metav1.Condition{
				Type:               RegistryMigrationStatusConditionType,
				Status:             metav1.ConditionFalse,
				Reason:             "MigrationJobFailed",
				ObservedGeneration: in.Generation,
			})
			return fmt.Errorf("registry migration job failed")
		}
		// TODO: handle other conditions
		return nil
	}

	// create the migration job
	migrationJob, err = newMigrationJob(in, cli)
	if err != nil {
		in.Status.SetCondition(metav1.Condition{
			Type:               RegistryMigrationStatusConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             "MigrationJobFailedBuild",
			ObservedGeneration: in.Generation,
		})
		return fmt.Errorf("build migration job: %w", err)
	}

	if err := cli.Create(ctx, &migrationJob); err != nil {
		in.Status.SetCondition(metav1.Condition{
			Type:               RegistryMigrationStatusConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             "MigrationJobFailedCreation",
			ObservedGeneration: in.Generation,
		})
		return fmt.Errorf("create migration job: %w", err)
	}

	in.Status.SetCondition(metav1.Condition{
		Type:               RegistryMigrationStatusConditionType,
		Status:             metav1.ConditionFalse,
		Reason:             "MigrationJobInProgress",
		ObservedGeneration: in.Generation,
	})

	return nil
}

// HasRegistryMigrated checks if the registry data has been migrated by looking for the 'migration complete' secret in the registry namespace
func HasRegistryMigrated(ctx context.Context, cli client.Client) (bool, error) {
	sec := corev1.Secret{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: registryNamespace, Name: RegistryDataMigrationCompleteSecretName}, &sec)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get registry migration secret: %w", err)
	}

	err = maybeDeleteRegistryJob(ctx, cli)
	if err != nil {
		return false, fmt.Errorf("cleanup registry migration job: %w", err)
	}

	return true, nil
}

func newMigrationJob(in *clusterv1beta1.Installation, cli client.Client) (batchv1.Job, error) {
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      registryDataMigrationJobName,
			Namespace: registryNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: RegistryMigrationServiceAccountName,
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
							Image:   os.Getenv("EMBEDDEDCLUSTER_IMAGE"),
							Command: []string{"/manager"},
							Args:    []string{`--migration=registry-data`},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "registry-data",
									MountPath: "/var/lib/embedded-cluster/registry",
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: registryS3SecretName,
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

	err := ctrl.SetControllerReference(in, &job, cli.Scheme())
	if err != nil {
		return batchv1.Job{}, fmt.Errorf("set controller reference: %w", err)
	}

	job.ObjectMeta.Labels = applyRegistryLabels(job.ObjectMeta.Labels, registryDataMigrationJobName)

	return job, nil
}

func maybeDeleteRegistryJob(ctx context.Context, cli client.Client) error {
	jo := batchv1.Job{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: registryNamespace, Name: registryDataMigrationJobName}, &jo)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // the job is done, nothing to delete
		}
		return fmt.Errorf("get registry migration job: %w", err)
	}

	err = cli.Delete(ctx, &jo)
	if err != nil {
		return fmt.Errorf("delete registry migration job: %w", err)
	}
	return nil
}
