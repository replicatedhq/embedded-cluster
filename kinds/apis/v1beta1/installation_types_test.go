package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

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
				RuntimeConfig: &RuntimeConfigSpec{
					AdminConsole: AdminConsoleSpec{
						Port: 31111,
					},
				},
				AdminConsole: &AdminConsoleSpec{
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
				RuntimeConfig: &RuntimeConfigSpec{
					LocalArtifactMirror: LocalArtifactMirrorSpec{
						Port: 51111,
					},
				},
				LocalArtifactMirror: &LocalArtifactMirrorSpec{
					Port: 51111,
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
