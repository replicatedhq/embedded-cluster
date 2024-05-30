package registry

import (
	"context"
	"encoding/json"
	"fmt"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/k8sutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// seaweedfsS3SecretName is the name of the Seaweedfs secret.
	// This secret name is defined in the chart in the release metadata.
	seaweedfsS3SecretName = "secret-seaweedfs-s3"

	// registryS3SecretName is the name of the Registry secret.
	// This secret name is defined in the chart in the release metadata.
	registryS3SecretName = "seaweedfs-s3-rw"

	// seaweedfsS3SecretReadyConditionType represents the condition type that indicates status of
	// the Seaweedfs secret.
	seaweedfsS3SecretReadyConditionType = "SeaweedfsS3SecretReady"

	// registryS3SecretReadyConditionType represents the condition type that indicates status of
	// the Registry secret.
	registryS3SecretReadyConditionType = "RegistryS3SecretReady"
)

func ensureSeaweedfsS3Secret(ctx context.Context, in *clusterv1beta1.Installation, cli client.Client) (*seaweedfsConfig, controllerutil.OperationResult, error) {
	log := ctrl.LoggerFrom(ctx)

	op := controllerutil.OperationResultNone

	err := ensureSeaweedfsNamespace(ctx, cli)
	if err != nil {
		return nil, op, fmt.Errorf("ensure seaweedfs namespace: %w", err)
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: seaweedfsS3SecretName, Namespace: seaweedfsNamespace},
	}

	var config seaweedfsConfig

	op, err = ctrl.CreateOrUpdate(ctx, cli, obj, func() error {
		err := ctrl.SetControllerReference(in, obj, cli.Scheme())
		if err != nil {
			return fmt.Errorf("set controller reference: %w", err)
		}

		obj.ObjectMeta.Labels = k8sutil.ApplyCommonLabels(obj.ObjectMeta.Labels, in, "s3")

		if obj.Data != nil {
			err := json.Unmarshal(obj.Data["seaweedfs_s3_config"], &config)
			if err != nil {
				log.Error(err, "Unmarshal seaweedfs_s3_config failed, will recreate the secret")
			}
		}

		var changed bool
		if _, ok := config.getCredentials("anvAdmin"); !ok {
			config.Identities = append(config.Identities, seaweedfsIdentity{
				Name: "anvAdmin",
				Credentials: []seaweedfsIdentityCredential{{
					AccessKey: randString(20),
					SecretKey: randString(40),
				}},
				Actions: []string{"Admin", "Read", "Write"},
			})
			changed = true
		}
		if _, ok := config.getCredentials("anvReadOnly"); !ok {
			config.Identities = append(config.Identities, seaweedfsIdentity{
				Name: "anvReadOnly",
				Credentials: []seaweedfsIdentityCredential{{
					AccessKey: randString(20),
					SecretKey: randString(40),
				}},
				Actions: []string{"Read"},
			})
			changed = true
		}
		if !changed {
			return nil
		}

		configData, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("marshal seaweedfs_s3_config: %w", err)
		}

		if obj.Data == nil {
			obj.Data = make(map[string][]byte)
		}
		obj.Data["seaweedfs_s3_config"] = configData

		return nil
	})
	if err != nil {
		return nil, op, fmt.Errorf("create or update seaweedfs s3 secret: %w", err)
	}

	return &config, op, nil
}

func ensureRegistryS3Secret(ctx context.Context, in *clusterv1beta1.Installation, cli client.Client, sfsConfig *seaweedfsConfig) (controllerutil.OperationResult, error) {
	op := controllerutil.OperationResultNone

	sfsCreds, ok := sfsConfig.getCredentials("anvAdmin")
	if !ok {
		return op, fmt.Errorf("seaweedfs s3 anvAdmin credentials not found")
	}

	err := ensureRegistryNamespace(ctx, cli)
	if err != nil {
		return op, fmt.Errorf("ensure registry namespace: %w", err)
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: registryS3SecretName, Namespace: registryNamespace},
	}

	op, err = ctrl.CreateOrUpdate(ctx, cli, obj, func() error {
		err := ctrl.SetControllerReference(in, obj, cli.Scheme())
		if err != nil {
			return fmt.Errorf("set controller reference: %w", err)
		}

		obj.ObjectMeta.Labels = k8sutil.ApplyCommonLabels(obj.ObjectMeta.Labels, in, "registry")

		if obj.Data == nil {
			obj.Data = make(map[string][]byte)
		}
		obj.Data["s3AccessKey"] = []byte(sfsCreds.AccessKey)
		obj.Data["s3SecretKey"] = []byte(sfsCreds.SecretKey)

		return nil
	})
	if err != nil {
		return op, fmt.Errorf("create or update registry s3 secret: %w", err)
	}

	return op, nil
}

func getRegistryS3SecretReadyCondition(in *clusterv1beta1.Installation, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return getCondition(in, registryS3SecretReadyConditionType, status, reason, message)
}

func getSeaweedfsS3SecretReadyCondition(in *clusterv1beta1.Installation, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return getCondition(in, seaweedfsS3SecretReadyConditionType, status, reason, message)
}
