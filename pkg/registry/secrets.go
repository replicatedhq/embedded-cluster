package registry

import (
	"context"
	"encoding/json"
	"fmt"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// SeaweedfsNamespace is the namespace where the Seaweedfs secret is stored.
	// This namespace is defined in the chart in the release metadata.
	SeaweedfsNamespace = "seaweedfs"

	// SeaweedfsS3SecretName is the name of the Seaweedfs secret.
	// This secret name is defined in the chart in the release metadata.
	SeaweedfsS3SecretName = "secret-seaweedfs-s3"

	// RegistryNamespace is the namespace where the Registry secret is stored.
	// This namespace is defined in the chart in the release metadata.
	RegistryNamespace = "registry"

	// RegistryS3SecretName is the name of the Registry secret.
	// This secret name is defined in the chart in the release metadata.
	RegistryS3SecretName = "seaweedfs-s3-rw"

	// SeaweedfsS3SecretReadyConditionType represents the condition type that indicates status of
	// the Seaweedfs secret.
	SeaweedfsS3SecretReadyConditionType = "SeaweedfsS3SecretReady"

	// RegistryS3SecretReadyConditionType represents the condition type that indicates status of
	// the Registry secret.
	RegistryS3SecretReadyConditionType = "RegistryS3SecretReady"
)

func EnsureSecrets(ctx context.Context, in *clusterv1beta1.Installation, cli client.Client) error {
	log := ctrl.LoggerFrom(ctx)

	sfsConfig, op, err := ensureSeaweedfsS3Secret(ctx, in, cli)
	if err != nil {
		in.Status.SetCondition(getSeaweedfsS3SecretReadyCondition(in, metav1.ConditionFalse, "SecretFailed", err.Error()))
		return fmt.Errorf("ensure seaweedfs s3 secret: %w", err)
	} else if op != controllerutil.OperationResultNone {
		log.Info("Seaweedfs s3 secret changed", "operation", op)
	}
	in.Status.SetCondition(getSeaweedfsS3SecretReadyCondition(in, metav1.ConditionTrue, "SecretReady", ""))

	op, err = ensureRegistryS3Secret(ctx, in, cli, sfsConfig)
	if err != nil {
		in.Status.SetCondition(getRegistryS3SecretReadyCondition(in, metav1.ConditionFalse, "SecretFailed", err.Error()))
		return fmt.Errorf("ensure registry s3 secret: %w", err)
	} else if op != controllerutil.OperationResultNone {
		log.Info("Registry s3 secret changed", "operation", op)
	}
	in.Status.SetCondition(getRegistryS3SecretReadyCondition(in, metav1.ConditionTrue, "SecretReady", ""))

	return nil
}

func ensureSeaweedfsS3Secret(ctx context.Context, in *clusterv1beta1.Installation, cli client.Client) (*seaweedfsConfig, controllerutil.OperationResult, error) {
	log := ctrl.LoggerFrom(ctx)

	op := controllerutil.OperationResultNone

	err := ensureSeaweedfsNamespace(ctx, cli, SeaweedfsNamespace)
	if err != nil {
		return nil, op, fmt.Errorf("ensure seaweedfs namespace: %w", err)
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: SeaweedfsS3SecretName, Namespace: SeaweedfsNamespace},
	}

	var config seaweedfsConfig

	op, err = ctrl.CreateOrUpdate(ctx, cli, obj, func() error {
		err := ctrl.SetControllerReference(in, obj, cli.Scheme())
		if err != nil {
			return fmt.Errorf("set controller reference: %w", err)
		}

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

	err := ensureRegistryNamespace(ctx, cli, RegistryNamespace)
	if err != nil {
		return op, fmt.Errorf("ensure registry namespace: %w", err)
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: RegistryS3SecretName, Namespace: RegistryNamespace},
	}

	op, err = ctrl.CreateOrUpdate(ctx, cli, obj, func() error {
		err := ctrl.SetControllerReference(in, obj, cli.Scheme())
		if err != nil {
			return fmt.Errorf("set controller reference: %w", err)
		}

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

func ensureSeaweedfsNamespace(ctx context.Context, cli client.Client, namespace string) error {
	obj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}

	err := cli.Create(ctx, obj)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create seaweedfs namespace: %w", err)

	}

	return nil
}

func ensureRegistryNamespace(ctx context.Context, cli client.Client, namespace string) error {
	obj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}

	err := cli.Create(ctx, obj)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create registry namespace: %w", err)
	}

	return nil
}

func getRegistryS3SecretReadyCondition(in *clusterv1beta1.Installation, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return getCondition(in, RegistryS3SecretReadyConditionType, status, reason, message)
}

func getSeaweedfsS3SecretReadyCondition(in *clusterv1beta1.Installation, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return getCondition(in, SeaweedfsS3SecretReadyConditionType, status, reason, message)
}

func getCondition(in *clusterv1beta1.Installation, conditionType string, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: in.Generation,
	}
}
