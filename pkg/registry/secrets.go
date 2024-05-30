package registry

import (
	"context"
	"encoding/json"
	"fmt"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster-kinds/types"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// SeaweedfsS3SecretReadyConditionType represents the condition type that indicates status of
	// the Seaweedfs secret.
	SeaweedfsS3SecretReadyConditionType = "SeaweedfsS3SecretReady"

	// RegistryS3SecretReadyConditionType represents the condition type that indicates status of
	// the Registry secret.
	RegistryS3SecretReadyConditionType = "RegistryS3SecretReady"
)

func EnsureSecrets(ctx context.Context, in *clusterv1beta1.Installation, metadata *ectypes.ReleaseMetadata, cli client.Client) error {
	if in == nil || !in.Spec.AirGap || !in.Spec.HighAvailability {
		return nil
	}

	log := ctrl.LoggerFrom(ctx)

	config, op, err := ensureSeaweedfsS3Secret(ctx, in, metadata, cli)
	if err != nil {
		in.Status.SetCondition(metav1.Condition{
			Type:               SeaweedfsS3SecretReadyConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             "SecretFailed",
			Message:            err.Error(),
			ObservedGeneration: in.Generation,
		})
		if errors.IsFatalError(err) {
			log.Error(err, "Fatal error, will not retry")
			return nil
		}
		return fmt.Errorf("ensure seaweedfs s3 secret: %w", err)
	} else if op != controllerutil.OperationResultNone {
		log.Info("Seaweedfs s3 secret changed", "operation", op)
	}
	in.Status.SetCondition(metav1.Condition{
		Type:               SeaweedfsS3SecretReadyConditionType,
		Status:             metav1.ConditionTrue,
		Reason:             "SecretReady",
		ObservedGeneration: in.Generation,
	})

	op, err = ensureRegistryS3Secret(ctx, in, metadata, cli, config)
	if err != nil {
		in.Status.SetCondition(metav1.Condition{
			Type:               RegistryS3SecretReadyConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             "SecretFailed",
			Message:            err.Error(),
			ObservedGeneration: in.Generation,
		})
		if errors.IsFatalError(err) {
			log.Error(err, "Fatal error, will not retry")
			return nil
		}
		return fmt.Errorf("ensure registry s3 secret: %w", err)
	} else if op != controllerutil.OperationResultNone {
		log.Info("Registry s3 secret changed", "operation", op)
	}
	in.Status.SetCondition(metav1.Condition{
		Type:               RegistryS3SecretReadyConditionType,
		Status:             metav1.ConditionTrue,
		Reason:             "SecretReady",
		ObservedGeneration: in.Generation,
	})

	return nil
}

func ensureSeaweedfsS3Secret(ctx context.Context, in *clusterv1beta1.Installation, metadata *ectypes.ReleaseMetadata, cli client.Client) (*seaweedfsConfig, controllerutil.OperationResult, error) {
	log := ctrl.LoggerFrom(ctx)

	op := controllerutil.OperationResultNone

	namespace, err := getSeaweedfsNamespaceFromMetadata(metadata)
	if err != nil {
		err = errors.NewFatalError(fmt.Errorf("get seaweedfs namespace from metadata: %w", err))
		return nil, op, err
	}

	secretName, err := getSeaweedfsS3SecretNameFromMetadata(metadata)
	if err != nil {
		err = errors.NewFatalError(fmt.Errorf("get seaweedfs s3 secret name from metadata: %w", err))
		return nil, op, err
	}

	err = ensureSeaweedfsNamespace(ctx, cli, namespace)
	if err != nil {
		return nil, op, fmt.Errorf("ensure seaweedfs namespace: %w", err)
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
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

func ensureRegistryS3Secret(ctx context.Context, in *clusterv1beta1.Installation, metadata *ectypes.ReleaseMetadata, cli client.Client, sfsConfig *seaweedfsConfig) (controllerutil.OperationResult, error) {
	op := controllerutil.OperationResultNone

	sfsCreds, ok := sfsConfig.getCredentials("anvAdmin")
	if !ok {
		return op, fmt.Errorf("seaweedfs s3 anvAdmin credentials not found")
	}

	namespace, err := getRegistryNamespaceFromMetadata(metadata)
	if err != nil {
		// TODO: this is a fatal error, should we return it?
		return op, fmt.Errorf("get registry namespace from metadata: %w", err)
	}

	secretName, err := getRegistryS3SecretNameFromMetadata(metadata)
	if err != nil {
		// TODO: this is a fatal error, should we return it?
		return op, fmt.Errorf("get registry s3 secret name from metadata: %w", err)
	}

	err = ensureRegistryNamespace(ctx, cli, namespace)
	if err != nil {
		return op, fmt.Errorf("ensure registry namespace: %w", err)
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
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
