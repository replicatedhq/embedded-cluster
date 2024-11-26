package envtest

import (
	"context"
	"testing"

	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestGetLatestInstallation(t *testing.T) {
	type args struct {
		cli func(t *testing.T) client.Client
	}
	tests := []struct {
		name     string
		args     args
		wantName string
		wantErr  error
	}{
		{
			name: "basic",
			args: args{
				cli: func(t *testing.T) client.Client {
					testEnv := &envtest.Environment{
						CRDDirectoryPaths:     []string{"../../../operator/config/crd/bases"},
						ErrorIfCRDPathMissing: true,
					}
					cfg, err := testEnv.Start()
					require.NoError(t, err)
					t.Cleanup(func() { _ = testEnv.Stop() })

					scheme := runtime.NewScheme()
					embeddedclusterv1beta1.AddToScheme(scheme)

					cli, err := client.New(cfg, client.Options{Scheme: scheme})
					require.NoError(t, err)

					err = cli.Create(context.Background(), &embeddedclusterv1beta1.Installation{
						ObjectMeta: metav1.ObjectMeta{
							Name: "installation-1",
						},
						Spec: embeddedclusterv1beta1.InstallationSpec{
							Config: &embeddedclusterv1beta1.ConfigSpec{
								Version: "1.0.0",
							},
						},
					})
					require.NoError(t, err)

					return cli
				},
			},
			wantName: "installation-1",
		},
		{
			name: "not found should fail with ErrNoInstallations",
			args: args{
				cli: func(t *testing.T) client.Client {
					testEnv := &envtest.Environment{
						CRDDirectoryPaths:     []string{"../../../operator/config/crd/bases"},
						ErrorIfCRDPathMissing: true,
					}
					cfg, err := testEnv.Start()
					require.NoError(t, err)
					t.Cleanup(func() { _ = testEnv.Stop() })

					scheme := runtime.NewScheme()
					embeddedclusterv1beta1.AddToScheme(scheme)

					cli, err := client.New(cfg, client.Options{Scheme: scheme})
					require.NoError(t, err)

					return cli
				},
			},
			wantErr: kubeutils.ErrNoInstallations{},
		},
		{
			name: "no crd should fail with ErrNoInstallations",
			args: args{
				cli: func(t *testing.T) client.Client {
					// do not set up crd
					testEnv := &envtest.Environment{}
					cfg, err := testEnv.Start()
					require.NoError(t, err)
					t.Cleanup(func() { _ = testEnv.Stop() })

					scheme := runtime.NewScheme()
					embeddedclusterv1beta1.AddToScheme(scheme)

					cli, err := client.New(cfg, client.Options{Scheme: scheme})
					require.NoError(t, err)

					return cli
				},
			},
			wantErr: kubeutils.ErrNoInstallations{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := kubeutils.GetLatestInstallation(context.Background(), tt.args.cli(t))
			if tt.wantErr != nil {
				require.Equal(t, tt.wantErr, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantName, got.Name)
			}
		})
	}
}
