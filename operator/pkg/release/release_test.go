// Package release contains function to help finding things out about a given
// embedded cluster release. It is being kept here so if we decide to manage
// releases in a different way, we can easily change it.
package release

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_localMetadataFor(t *testing.T) {
	testMetadataJson := `{
		"Versions": {
			"EmbeddedClusterOperator": "v1.0.0"
		},
		"BuiltinConfigs": {
			"registry": {
				"Charts": [
					{
						"Name": "docker-registry",
						"Values": "ha: false\n"
					}
				]
			},
			"registry-ha": {
				"Charts": [
					{
						"Name": "docker-registry",
						"Values": "ha: true\n"
					}
				]
			}
		}
	}`

	type args struct {
		cli     client.Client
		version string
	}
	tests := []struct {
		name    string
		args    args
		want    *ectypes.ReleaseMetadata
		wantErr bool
	}{
		{
			name: "registry secret does not exist",
			args: args{
				cli: fake.NewClientBuilder().WithObjects(
					&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "version-metadata-no-tls",
							Namespace: "embedded-cluster",
						},
						Data: map[string]string{
							"metadata.json": testMetadataJson,
						},
					},
				).Build(),
				version: "no-tls",
			},
			want: &ectypes.ReleaseMetadata{
				Versions: map[string]string{
					"EmbeddedClusterOperator": "v1.0.0",
				},
				BuiltinConfigs: map[string]v1beta1.Helm{
					"registry": {
						Charts: []v1beta1.Chart{
							{
								Name:   "docker-registry",
								Values: "ha: false\n",
							},
						},
					},
					"registry-ha": {
						Charts: []v1beta1.Chart{
							{
								Name:   "docker-registry",
								Values: "ha: true\n",
							},
						},
					},
				},
			},
		},
		{
			name: "registry secret exists",
			args: args{
				cli: fake.NewClientBuilder().WithObjects(
					&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "version-metadata-with-tls",
							Namespace: "embedded-cluster",
						},
						Data: map[string]string{
							"metadata.json": testMetadataJson,
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "registry-tls",
							Namespace: "registry",
						},
					},
				).Build(),
				version: "with-tls",
			},
			want: &ectypes.ReleaseMetadata{
				Versions: map[string]string{
					"EmbeddedClusterOperator": "v1.0.0",
				},
				BuiltinConfigs: map[string]v1beta1.Helm{
					"registry": {
						Charts: []v1beta1.Chart{
							{
								Name:   "docker-registry",
								Values: "ha: false\ntlsSecretName: registry-tls\n",
							},
						},
					},
					"registry-ha": {
						Charts: []v1beta1.Chart{
							{
								Name:   "docker-registry",
								Values: "ha: true\ntlsSecretName: registry-tls\n",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			got, err := localMetadataFor(context.Background(), tt.args.cli, tt.args.version)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}
