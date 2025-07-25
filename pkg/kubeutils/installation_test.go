package kubeutils

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/google/uuid"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/crds"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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

func TestRecordInstallation(t *testing.T) {
	// Setup the test scheme
	s := runtime.NewScheme()
	require.NoError(t, apiextensionsv1.AddToScheme(s))
	require.NoError(t, ecv1beta1.AddToScheme(s))
	require.NoError(t, corev1.AddToScheme(s))

	tests := []struct {
		name     string
		opts     RecordInstallationOptions
		wantErr  bool
		validate func(t *testing.T, installation *ecv1beta1.Installation)
	}{
		{
			name: "online installation without airgap",
			opts: RecordInstallationOptions{
				ClusterID: uuid.New().String(),
				IsAirgap:  false,
				License: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						IsDisasterRecoverySupported:       true,
						IsEmbeddedClusterMultiNodeEnabled: false,
					},
				},
				ConfigSpec: &ecv1beta1.ConfigSpec{
					Version: "1.15.0+k8s-1.30",
				},
				MetricsBaseURL: "https://replicated.app",
				RuntimeConfig: &ecv1beta1.RuntimeConfigSpec{
					DataDir: "/var/lib/embedded-cluster",
				},
				EndUserConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{
						UnsupportedOverrides: ecv1beta1.UnsupportedOverrides{
							K0s: "apiVersion: k0s.k0sproject.io/v1beta1\nkind: Cluster",
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, installation *ecv1beta1.Installation) {
				assert.False(t, installation.Spec.AirGap)
				assert.Equal(t, int64(0), installation.Spec.AirgapUncompressedSize)
				assert.Equal(t, "1.15.0+k8s-1.30", installation.Spec.Config.Version)
				assert.Equal(t, "https://replicated.app", installation.Spec.MetricsBaseURL)
				assert.Equal(t, "/var/lib/embedded-cluster", installation.Spec.RuntimeConfig.DataDir)
				assert.Equal(t, "apiVersion: k0s.k0sproject.io/v1beta1\nkind: Cluster", installation.Spec.EndUserK0sConfigOverrides)
				assert.True(t, installation.Spec.LicenseInfo.IsDisasterRecoverySupported)
				assert.False(t, installation.Spec.LicenseInfo.IsMultiNodeEnabled)
				assert.Equal(t, ecv1beta1.InstallationStateKubernetesInstalled, installation.Status.State)
				assert.Equal(t, "Kubernetes installed", installation.Status.Reason)
			},
		},
		{
			name: "airgap installation with uncompressed size",
			opts: RecordInstallationOptions{
				ClusterID: uuid.New().String(),
				IsAirgap:  true,
				License: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						IsDisasterRecoverySupported:       false,
						IsEmbeddedClusterMultiNodeEnabled: true,
					},
				},
				ConfigSpec: &ecv1beta1.ConfigSpec{
					Version: "1.16.0+k8s-1.31",
				},
				MetricsBaseURL: "https://staging.replicated.app",
				RuntimeConfig: &ecv1beta1.RuntimeConfigSpec{
					DataDir: "/opt/embedded-cluster",
				},
				EndUserConfig:          nil,
				AirgapUncompressedSize: 1234567890,
			},
			wantErr: false,
			validate: func(t *testing.T, installation *ecv1beta1.Installation) {
				assert.True(t, installation.Spec.AirGap)
				assert.Equal(t, int64(1234567890), installation.Spec.AirgapUncompressedSize)
				assert.Equal(t, "1.16.0+k8s-1.31", installation.Spec.Config.Version)
				assert.Equal(t, "https://staging.replicated.app", installation.Spec.MetricsBaseURL)
				assert.Equal(t, "/opt/embedded-cluster", installation.Spec.RuntimeConfig.DataDir)
				assert.Empty(t, installation.Spec.EndUserK0sConfigOverrides)
				assert.False(t, installation.Spec.LicenseInfo.IsDisasterRecoverySupported)
				assert.True(t, installation.Spec.LicenseInfo.IsMultiNodeEnabled)
				assert.Equal(t, ecv1beta1.InstallationStateKubernetesInstalled, installation.Status.State)
				assert.Equal(t, "Kubernetes installed", installation.Status.Reason)
			},
		},
		{
			name: "airgap installation with large uncompressed size",
			opts: RecordInstallationOptions{
				ClusterID: uuid.New().String(),
				IsAirgap:  true,
				License: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						IsDisasterRecoverySupported:       false,
						IsEmbeddedClusterMultiNodeEnabled: false,
					},
				},
				ConfigSpec: &ecv1beta1.ConfigSpec{
					Version: "1.18.0+k8s-1.33",
				},
				MetricsBaseURL: "https://custom.replicated.app",
				RuntimeConfig: &ecv1beta1.RuntimeConfigSpec{
					DataDir: "/custom/data/dir",
				},
				EndUserConfig:          nil,
				AirgapUncompressedSize: 9876543210,
			},
			wantErr: false,
			validate: func(t *testing.T, installation *ecv1beta1.Installation) {
				assert.True(t, installation.Spec.AirGap)
				assert.Equal(t, int64(9876543210), installation.Spec.AirgapUncompressedSize)
				assert.Equal(t, "1.18.0+k8s-1.33", installation.Spec.Config.Version)
				assert.Equal(t, "https://custom.replicated.app", installation.Spec.MetricsBaseURL)
				assert.Equal(t, "/custom/data/dir", installation.Spec.RuntimeConfig.DataDir)
				assert.Empty(t, installation.Spec.EndUserK0sConfigOverrides)
				assert.False(t, installation.Spec.LicenseInfo.IsDisasterRecoverySupported)
				assert.False(t, installation.Spec.LicenseInfo.IsMultiNodeEnabled)
				assert.Equal(t, ecv1beta1.InstallationStateKubernetesInstalled, installation.Status.State)
				assert.Equal(t, "Kubernetes installed", installation.Status.Reason)
			},
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
			installation, err := RecordInstallation(ctx, cli, tt.opts)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, installation)

			// Verify the installation was created in the cluster
			var resultInstallation ecv1beta1.Installation
			err = cli.Get(ctx, client.ObjectKey{Name: installation.Name}, &resultInstallation)
			require.NoError(t, err)

			// Run custom validation
			if tt.validate != nil {
				tt.validate(t, &resultInstallation)
			}

			json, err := json.MarshalIndent(resultInstallation, "", "  ")
			require.NoError(t, err)
			t.Logf("resultInstallation: %s", string(json))
			// Verify common fields
			assert.NotEmpty(t, resultInstallation.Name)
			assert.Equal(t, "", resultInstallation.APIVersion) // I expected this to be "embeddedcluster.replicated.com/v1beta1"
			assert.Equal(t, "", resultInstallation.Kind)       // I expected this to be "Installation"
			assert.Equal(t, tt.opts.ClusterID, resultInstallation.Spec.ClusterID)
			assert.Equal(t, ecv1beta1.InstallationSourceTypeCRD, resultInstallation.Spec.SourceType)
			assert.Equal(t, "ec-install", resultInstallation.Labels["replicated.com/disaster-recovery"])
		})
	}
}
