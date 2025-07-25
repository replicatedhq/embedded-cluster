package integration

import (
	"context"
	_ "embed"
	"strings"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func NewTestControllerNode(hostname string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.ToLower(hostname),
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func NewTestInterceptorFuncs() interceptor.Funcs {
	return interceptor.Funcs{
		Create: func(ctx context.Context, cli client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition); ok {
				err := cli.Create(ctx, obj, opts...)
				if err != nil {
					return err
				}
				// Update status to ready after creation
				crd.Status.Conditions = []apiextensionsv1.CustomResourceDefinitionCondition{
					{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue},
					{Type: apiextensionsv1.NamesAccepted, Status: apiextensionsv1.ConditionTrue},
				}
				return cli.Status().Update(ctx, crd)
			}
			return cli.Create(ctx, obj, opts...)
		},
	}
}

func NewAPIWithReleaseData(t *testing.T, opts ...api.Option) *api.API {
	cfg := types.APIConfig{
		Password:    "password",
		ReleaseData: DefaultReleaseData(),
	}
	a, err := api.New(cfg, opts...)
	require.NoError(t, err)
	return a
}

func DefaultReleaseData() *release.ReleaseData {
	return &release.ReleaseData{
		AppConfig: &kotsv1beta1.Config{},
	}
}
