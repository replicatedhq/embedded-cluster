package registry

import (
	"context"
	"fmt"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// seaweedfsNamespace is the namespace where the Seaweedfs chart is installed.
	// This namespace is defined in the chart in the release metadata.
	seaweedfsNamespace = "seaweedfs"

	// seaweedfsS3SVCName is the name of the Seaweedfs S3 service managed by the operator.
	// HACK: This service has a hardcoded service IP shared by the cli and operator as it is used
	// by the registry to redirect requests for blobs.
	seaweedfsS3SVCName = "ec-seaweedfs-s3"

	// seaweedfsS3ServiceReadyConditionType represents the condition type that indicates status of
	// the Seaweedfs service.
	seaweedfsS3ServiceReadyConditionType = "SeaweedfsS3ServiceReady"

	// seaweedfsLowerBandIPIndex is the index of the seaweedfs service IP in the service CIDR.
	// HACK: this is shared with the cli and operator as it is used by the registry to redirect requests for blobs.
	seaweedfsLowerBandIPIndex = 11
)

func GetSeaweedfsS3Endpoint(serviceCIDR string) (string, error) {
	ip, err := getSeaweedfsS3ServiceIP(serviceCIDR)
	if err != nil {
		return "", fmt.Errorf("get seaweedfs s3 service IP: %w", err)
	}
	return fmt.Sprintf("%s:8333", ip), nil
}

func getSeaweedfsS3ServiceIP(serviceCIDR string) (string, error) {
	ip, err := util.GetLowerBandIP(serviceCIDR, seaweedfsLowerBandIPIndex)
	if err != nil {
		return "", fmt.Errorf("get lower band ip at index %d: %w", seaweedfsLowerBandIPIndex, err)
	}
	return ip.String(), nil
}

func ensureSeaweedfsS3Service(ctx context.Context, in *clusterv1beta1.Installation, cli client.Client, clusterIP string) (controllerutil.OperationResult, error) {
	op := controllerutil.OperationResultNone

	err := ensureSeaweedfsNamespace(ctx, cli)
	if err != nil {
		return op, fmt.Errorf("ensure seaweedfs namespace: %w", err)
	}

	obj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: seaweedfsS3SVCName, Namespace: seaweedfsNamespace},
	}

	op, err = ctrl.CreateOrUpdate(ctx, cli, obj, func() error {
		err := ctrl.SetControllerReference(in, obj, cli.Scheme())
		if err != nil {
			return fmt.Errorf("set controller reference: %w", err)
		}

		obj.ObjectMeta.Labels = k8sutil.ApplyCommonLabels(obj.ObjectMeta.Labels, in, "s3")

		obj.Spec.ClusterIP = clusterIP
		obj.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "swfs-s3",
				Port:       8333,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(8333),
			},
		}
		obj.Spec.Selector = map[string]string{
			"app.kubernetes.io/component": "filer",
			"app.kubernetes.io/name":      "seaweedfs",
		}

		return nil
	})
	if err != nil {
		return op, fmt.Errorf("create or update registry s3 service: %w", err)
	}

	return op, nil
}

func ensureSeaweedfsNamespace(ctx context.Context, cli client.Client) error {
	obj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: seaweedfsNamespace},
	}

	err := cli.Create(ctx, obj)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create seaweedfs namespace: %w", err)

	}

	return nil
}

func getSeaweedfsS3ServiceReadyCondition(in *clusterv1beta1.Installation, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return getCondition(in, seaweedfsS3ServiceReadyConditionType, status, reason, message)
}
