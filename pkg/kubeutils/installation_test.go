package kubeutils

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/crds"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/yaml"
)

func Test_lessThanECVersion115(t *testing.T) {
	type args struct {
		ver *semver.Version
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "less than 1.15",
			args: args{
				ver: semver.MustParse("1.14.0+k8s-1.29-49-gf92daca6"),
			},
			want: true,
		},
		{
			name: "greater than or equal to 1.15",
			args: args{
				ver: semver.MustParse("1.15.0+k8s-1.29-49-gf92daca6"),
			},
			want: false,
		},
		{
			name: "old version scheme",
			args: args{
				ver: semver.MustParse("1.28.7+ec.0"),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lessThanECVersion115(tt.args.ver); got != tt.want {
				t.Errorf("lessThanECVersion115() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureInstallationCRD(t *testing.T) {
	// Setup the test scheme
	s := runtime.NewScheme()
	require.NoError(t, apiextensionsv1.AddToScheme(s))
	require.NoError(t, ecv1beta1.AddToScheme(s))

	// Parse CRD for testing
	var testCRD apiextensionsv1.CustomResourceDefinition
	crds := strings.Split(crds.InstallationCRDFile, "\n---\n")
	require.NotEmpty(t, crds)
	require.NoError(t, yaml.Unmarshal([]byte(crds[0]), &testCRD))
	crdName := testCRD.Name

	tests := []struct {
		name            string
		existingObjects []runtime.Object
		wantErr         bool
	}{
		{
			name:            "create new CRD",
			existingObjects: []runtime.Object{},
			wantErr:         false,
		},
		{
			name: "update existing CRD",
			existingObjects: []runtime.Object{
				&apiextensionsv1.CustomResourceDefinition{
					TypeMeta:   testCRD.TypeMeta,
					ObjectMeta: testCRD.ObjectMeta,
					// Different spec to force update
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: testCRD.Spec.Group,
						Names: testCRD.Spec.Names,
						Scope: testCRD.Spec.Scope,
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name: "v1beta1",
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
										Type: "object",
									},
								},
								Served:  true,
								Storage: true,
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup the test environment
			verbosity := 1
			if os.Getenv("DEBUG") != "" {
				verbosity = 10
			}
			log := testr.NewWithOptions(t, testr.Options{Verbosity: verbosity})
			ctx := logr.NewContext(context.Background(), log)

			testEnv := &envtest.Environment{}
			cfg, err := testEnv.Start()
			require.NoError(t, err)
			t.Cleanup(func() { _ = testEnv.Stop() })

			cli, err := client.New(cfg, client.Options{Scheme: s})
			require.NoError(t, err)

			// Call the function being tested
			err = EnsureInstallationCRD(ctx, cli)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Create an installation object
			installation := &ecv1beta1.Installation{
				TypeMeta: metav1.TypeMeta{
					APIVersion: ecv1beta1.GroupVersion.String(),
					Kind:       "Installation",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: time.Now().Format("20060102150405"),
				},
				Spec: ecv1beta1.InstallationSpec{
					ClusterID:  "test-cluster-id",
					SourceType: ecv1beta1.InstallationSourceTypeCRD,
				},
			}

			// Create the installation
			err = cli.Create(ctx, installation)
			require.NoError(t, err)

			// Verify installation was created
			var resultInstallation ecv1beta1.Installation
			err = cli.Get(ctx, client.ObjectKey{Name: installation.Name}, &resultInstallation)
			require.NoError(t, err)
			require.Equal(t, "test-cluster-id", resultInstallation.Spec.ClusterID)

			// Verify CRD was created or updated
			var resultCRD apiextensionsv1.CustomResourceDefinition
			err = cli.Get(ctx, client.ObjectKey{Name: crdName}, &resultCRD)
			require.NoError(t, err)

			// Verify labels and annotations for creation path
			if len(tt.existingObjects) == 0 {
				require.Equal(t, "Helm", resultCRD.Labels["app.kubernetes.io/managed-by"])
				require.Equal(t, "embedded-cluster-operator", resultCRD.Annotations["meta.helm.sh/release-name"])
				require.Equal(t, "embedded-cluster", resultCRD.Annotations["meta.helm.sh/release-namespace"])
			}
		})
	}
}
