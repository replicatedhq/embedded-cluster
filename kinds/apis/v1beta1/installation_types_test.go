package v1beta1

import (
	"embed"
	_ "embed"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	k8syaml "sigs.k8s.io/yaml"
)

//go:embed testdata/*
var testData embed.FS

func TestParseConfigSpecFromSecret(t *testing.T) {
	type test struct {
		Name       string            `yaml:"name"`
		SecretData map[string]string `yaml:"secret"`
		ConfigSpec *ConfigSpec       `yaml:"configSpec"`
		Expected   *ConfigSpec       `yaml:"expected"`
		Error      string            `yaml:"error"`
	}

	for tname, tt := range parseTestsYAML[test](t, "config-override-") {
		t.Run(tname, func(t *testing.T) {
			in := &InstallationSpec{Config: tt.ConfigSpec}
			secret := v1.Secret{
				Data: map[string][]byte{},
			}
			for k, v := range tt.SecretData {
				secret.Data[k] = []byte(v)
			}
			if err := in.ParseConfigSpecFromSecret(secret); err != nil {
				require.NotEmpty(t, tt.Error, "unexpected error: %v", err)
				require.Contains(t, err.Error(), tt.Error)
				return
			}
			require.Empty(t, tt.Error, "expected error: %v", tt.Error)
			require.Equal(t, tt.Expected, in.Config)
		})
	}
}

func TestInstallationSpec_UnmarshalJSON(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name string
		args args
		want InstallationSpec
	}{
		{
			name: "admin console port",
			args: args{
				in: `
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Installation
metadata:
  name: test
spec:
  config:
    version: 1.29.1+k0s.0
  adminConsole:
    port: 31111
`,
			},
			want: InstallationSpec{
				Config: &ConfigSpec{
					Version: "1.29.1+k0s.0",
				},
				SourceType: InstallationSourceTypeCRD,
				RuntimeConfig: &RuntimeConfigSpec{
					AdminConsole: AdminConsoleSpec{
						Port: 31111,
					},
				},
				Deprecated_AdminConsole: &AdminConsoleSpec{
					Port: 31111,
				},
			},
		},
		{
			name: "local artifact mirror port",
			args: args{
				in: `
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Installation
metadata:
  name: test
spec:
  config:
    version: 1.29.1+k0s.0
  localArtifactMirror:
    port: 51111
`,
			},
			want: InstallationSpec{
				Config: &ConfigSpec{
					Version: "1.29.1+k0s.0",
				},
				SourceType: InstallationSourceTypeCRD,
				RuntimeConfig: &RuntimeConfigSpec{
					LocalArtifactMirror: LocalArtifactMirrorSpec{
						Port: 51111,
					},
				},
				Deprecated_LocalArtifactMirror: &LocalArtifactMirrorSpec{
					Port: 51111,
				},
			},
		},
		{
			name: "proxy configuration",
			args: args{
				in: `
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Installation
metadata:
  name: test
spec:
  config:
    version: 1.29.1+k0s.0
  proxy:
    httpProxy: http://proxy.example.com:8080
    httpsProxy: https://proxy.example.com:8443
    noProxy: localhost,127.0.0.1
`,
			},
			want: InstallationSpec{
				Config: &ConfigSpec{
					Version: "1.29.1+k0s.0",
				},
				SourceType: InstallationSourceTypeCRD,
				RuntimeConfig: &RuntimeConfigSpec{
					Proxy: &ProxySpec{
						HTTPProxy:  "http://proxy.example.com:8080",
						HTTPSProxy: "https://proxy.example.com:8443",
						NoProxy:    "localhost,127.0.0.1",
					},
				},
				Deprecated_Proxy: &ProxySpec{
					HTTPProxy:  "http://proxy.example.com:8080",
					HTTPSProxy: "https://proxy.example.com:8443",
					NoProxy:    "localhost,127.0.0.1",
				},
			},
		},
		{
			name: "network configuration",
			args: args{
				in: `
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Installation
metadata:
  name: test
spec:
  config:
    version: 1.29.1+k0s.0
  network:
    podCIDR: 10.244.0.0/16
    serviceCIDR: 10.96.0.0/12
    nodePortRange: 30000-32767
`,
			},
			want: InstallationSpec{
				Config: &ConfigSpec{
					Version: "1.29.1+k0s.0",
				},
				SourceType: InstallationSourceTypeCRD,
				RuntimeConfig: &RuntimeConfigSpec{
					Network: NetworkSpec{
						PodCIDR:       "10.244.0.0/16",
						ServiceCIDR:   "10.96.0.0/12",
						NodePortRange: "30000-32767",
					},
				},
				Deprecated_Network: &NetworkSpec{
					PodCIDR:       "10.244.0.0/16",
					ServiceCIDR:   "10.96.0.0/12",
					NodePortRange: "30000-32767",
				},
			},
		},
	}
	for _, tt := range tests {
		scheme := runtime.NewScheme()
		err := AddToScheme(scheme)
		require.NoError(t, err)
		decode := serializer.NewCodecFactory(scheme).UniversalDeserializer().Decode

		t.Run(tt.name, func(t *testing.T) {
			obj, _, err := decode([]byte(tt.args.in), nil, nil)
			require.NoError(t, err)

			got, ok := obj.(*Installation)
			if !ok {
				t.Fatalf("expected Installation, got %T", obj)
			}
			assert.Equal(t, tt.want, got.Spec)
		})
	}
}

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
		err = k8syaml.Unmarshal(data, &onetest)
		require.NoError(t, err)

		tests[fpath] = onetest
	}
	return tests
}
