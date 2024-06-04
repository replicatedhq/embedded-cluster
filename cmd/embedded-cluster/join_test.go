package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k0sproject/dig"
	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	k8syaml "sigs.k8s.io/yaml"
)

//go:embed testdata/*
var testData embed.FS

func parseTestsYAML[T any](t *testing.T, prefix string) map[string]T {
	entries, err := testData.ReadDir("testdata")
	require.NoError(t, err)
	tests := make(map[string]T, 0)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}

		fpath := filepath.Join("testdata", entry.Name())
		data, err := testData.ReadFile(fpath)
		require.NoError(t, err)

		var onetest T
		err = yaml.Unmarshal(data, &onetest)
		require.NoError(t, err)

		tests[fpath] = onetest
	}
	return tests
}

func Test_patchK0sConfig(t *testing.T) {
	type test struct {
		Name     string
		Original string `yaml:"original"`
		Override string `yaml:"override"`
		Expected string `yaml:"expected"`
	}
	for tname, tt := range parseTestsYAML[test](t, "patch-k0s-config-") {
		t.Run(tname, func(t *testing.T) {
			req := require.New(t)

			originalFile, err := os.CreateTemp("", "k0s-original-*.yaml")
			req.NoError(err, "unable to create temp file")
			defer func() {
				originalFile.Close()
				os.Remove(originalFile.Name())
			}()
			err = os.WriteFile(originalFile.Name(), []byte(tt.Original), 0644)
			req.NoError(err, "unable to write original config")

			var patch string
			if tt.Override != "" {
				var overrides embeddedclusterv1beta1.Config
				err = k8syaml.Unmarshal([]byte(tt.Override), &overrides)
				req.NoError(err, "unable to unmarshal override")
				patch = overrides.Spec.UnsupportedOverrides.K0s
			}

			err = patchK0sConfig(originalFile.Name(), patch)
			req.NoError(err, "unable to patch config")

			data, err := os.ReadFile(originalFile.Name())
			req.NoError(err, "unable to read patched config")

			var original k0sconfig.ClusterConfig
			err = k8syaml.Unmarshal(data, &original)
			req.NoError(err, "unable to decode original file")

			var expected k0sconfig.ClusterConfig
			err = k8syaml.Unmarshal([]byte(tt.Expected), &expected)
			req.NoError(err, "unable to unmarshal expected file")

			assert.Equal(t, expected, original)
		})
	}
}

func TestJoinCommandResponseOverrides(t *testing.T) {
	type test struct {
		Name                      string
		EmbeddedOverrides         string `yaml:"embeddedOverrides"`
		EndUserOverrides          string `yaml:"endUserOverrides"`
		ExpectedEmbeddedOverrides string `yaml:"expectedEmbeddedOverrides"`
		ExpectedUserOverrides     string `yaml:"expectedUserOverrides"`
	}
	for tname, tt := range parseTestsYAML[test](t, "join-command-response-") {
		t.Run(tname, func(t *testing.T) {
			req := require.New(t)
			join := JoinCommandResponse{
				K0sUnsupportedOverrides:   tt.EmbeddedOverrides,
				EndUserK0sConfigOverrides: tt.EndUserOverrides,
			}

			embedded, err := join.EmbeddedOverrides()
			req.NoError(err, "unable to patch config")
			expectedEmbedded := dig.Mapping{}
			err = yaml.Unmarshal([]byte(tt.ExpectedEmbeddedOverrides), &expectedEmbedded)
			req.NoError(err, "unable to unmarshal expected file")
			embeddedStr := fmt.Sprintf("%+v", embedded)
			expectedEmbeddedStr := fmt.Sprintf("%+v", expectedEmbedded)
			assert.Equal(t, expectedEmbeddedStr, embeddedStr)

			user, err := join.EndUserOverrides()
			req.NoError(err, "unable to patch config")
			expectedUser := dig.Mapping{}
			err = yaml.Unmarshal([]byte(tt.ExpectedUserOverrides), &expectedUser)
			req.NoError(err, "unable to unmarshal expected file")
			userStr := fmt.Sprintf("%+v", user)
			expectedUserStr := fmt.Sprintf("%+v", expectedUser)
			assert.Equal(t, expectedUserStr, userStr)
		})
	}
}

func Test_canEnableHA(t *testing.T) {
	scheme := scheme.Scheme
	embeddedclusterv1beta1.AddToScheme(scheme)
	controllerLabels := map[string]string{"node-role.kubernetes.io/control-plane": "true"}
	type args struct {
		kcli client.Client
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "high availability is not enabled and there is three or more controller nodes",
			args: args{
				kcli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&embeddedclusterv1beta1.Installation{
						ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
						Spec:       embeddedclusterv1beta1.InstallationSpec{HighAvailability: false},
					},
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: controllerLabels}},
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: controllerLabels}},
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node3", Labels: controllerLabels}},
				).Build(),
			},
			want: true,
		},
		{
			name: "high availability is not enabled and there is not three or more controller nodes",
			args: args{
				kcli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&embeddedclusterv1beta1.Installation{
						ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
						Spec:       embeddedclusterv1beta1.InstallationSpec{HighAvailability: false},
					},
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: controllerLabels}},
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: controllerLabels}},
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node3"}},
				).Build(),
			},
			want: false,
		},
		{
			name: "high availability is already enabled",
			args: args{
				kcli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&embeddedclusterv1beta1.Installation{
						ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
						Spec:       embeddedclusterv1beta1.InstallationSpec{HighAvailability: true},
					},
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: controllerLabels}},
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: controllerLabels}},
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node3", Labels: controllerLabels}},
				).Build(),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			ctx := context.Background()
			got, err := canEnableHA(ctx, tt.args.kcli)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}

func Test_enableHA(t *testing.T) {
	scheme := scheme.Scheme
	embeddedclusterv1beta1.AddToScheme(scheme)
	type args struct {
		kcli client.Client
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "happy path",
			args: args{
				kcli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&embeddedclusterv1beta1.Installation{
						ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
						Spec:       embeddedclusterv1beta1.InstallationSpec{HighAvailability: false},
						Status: embeddedclusterv1beta1.InstallationStatus{
							Conditions: []metav1.Condition{
								{
									Type:   registry.RegistryMigrationStatusConditionType,
									Status: metav1.ConditionTrue,
									Reason: "MigrationJobCompleted",
								},
							},
							State: embeddedclusterv1beta1.InstallationStateInstalled,
						},
					},
				).Build(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			ctx := context.Background()
			err := enableHA(ctx, tt.args.kcli)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
			// validate that high availability is enabled
			var installation embeddedclusterv1beta1.Installation
			err = tt.args.kcli.Get(ctx, client.ObjectKey{Name: "test-installation"}, &installation)
			req.NoError(err)
			req.True(installation.Spec.HighAvailability)
		})
	}
}
