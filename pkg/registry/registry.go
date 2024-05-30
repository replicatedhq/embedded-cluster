package registry

import (
	"context"
	"fmt"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// registryNamespace is the namespace where the Registry secret is stored.
	// This namespace is defined in the chart in the release metadata.
	registryNamespace = "registry"

	// seaweedfsLowerBandIPIndex is the index of the seaweedfs service IP in the service CIDR.
	// HACK: this is shared with the cli and operator as it is used by the registry to redirect requests for blobs.
	seaweedfsLowerBandIPIndex = 11
)

func EnsureResources(ctx context.Context, in *clusterv1beta1.Installation, cli client.Client, serviceCIDR string) error {
	log := ctrl.LoggerFrom(ctx)

	sfsConfig, op, err := ensureSeaweedfsS3Secret(ctx, in, cli)
	if err != nil {
		in.Status.SetCondition(getSeaweedfsS3SecretReadyCondition(in, metav1.ConditionFalse, "Failed", err.Error()))
		return fmt.Errorf("ensure seaweedfs s3 secret: %w", err)
	} else if op != controllerutil.OperationResultNone {
		log.Info("Seaweedfs s3 secret changed", "operation", op)
	}
	in.Status.SetCondition(getSeaweedfsS3SecretReadyCondition(in, metav1.ConditionTrue, "SecretReady", ""))

	op, err = ensureRegistryS3Secret(ctx, in, cli, sfsConfig)
	if err != nil {
		in.Status.SetCondition(getRegistryS3SecretReadyCondition(in, metav1.ConditionFalse, "Failed", err.Error()))
		return fmt.Errorf("ensure registry s3 secret: %w", err)
	} else if op != controllerutil.OperationResultNone {
		log.Info("Registry s3 secret changed", "operation", op)
	}
	in.Status.SetCondition(getRegistryS3SecretReadyCondition(in, metav1.ConditionTrue, "SecretReady", ""))

	seaweedfsS3ServiceIP, err := util.GetLowerBandIP(serviceCIDR, seaweedfsLowerBandIPIndex)
	if err != nil {
		err = fmt.Errorf("get cluster IP for seaweedfs s3 service: %w", err)
		in.Status.SetCondition(getSeaweedfsS3ServiceReadyCondition(in, metav1.ConditionFalse, "Failed", err.Error()))
		return err
	}

	op, err = ensureSeaweedfsS3Service(ctx, in, cli, seaweedfsS3ServiceIP.String())
	if err != nil {
		in.Status.SetCondition(getSeaweedfsS3ServiceReadyCondition(in, metav1.ConditionFalse, "Failed", err.Error()))
		return fmt.Errorf("ensure seaweedfs s3 service: %w", err)
	} else if op != controllerutil.OperationResultNone {
		log.Info("Seaweedfs s3 service changed", "operation", op)
	}
	in.Status.SetCondition(getSeaweedfsS3ServiceReadyCondition(in, metav1.ConditionTrue, "ServiceReady", ""))

	return nil
}

func ensureRegistryNamespace(ctx context.Context, cli client.Client) error {
	obj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: registryNamespace},
	}

	err := cli.Create(ctx, obj)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create registry namespace: %w", err)
	}

	return nil
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
