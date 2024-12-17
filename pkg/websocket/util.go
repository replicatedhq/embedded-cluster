package websocket

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getKOTSClusterIP(ctx context.Context, kcli client.Client) (string, error) {
	var svc corev1.Service
	if err := kcli.Get(ctx, types.NamespacedName{Name: "kotsadm", Namespace: runtimeconfig.KotsadmNamespace}, &svc); err != nil {
		return "", errors.Wrap(err, "get kotsadm service")
	}
	if svc.Spec.ClusterIP == "" {
		return "", errors.New("cluster ip is empty")
	}
	return svc.Spec.ClusterIP, nil
}
