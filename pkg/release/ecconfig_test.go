package release

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateApp(t *testing.T) {
	tests := []struct {
		name        string
		releaseData map[string][]byte
		wantErr     string
	}{
		{
			name: "valid",
			releaseData: map[string][]byte{
				"embedded-cluster-config.yaml": []byte(`
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
metadata:
  name: "testconfig"`)},
		},
		{
			name: "bad extension values",
			releaseData: map[string][]byte{
				"embedded-cluster-config.yaml": []byte(`
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
metadata:
  name: "testconfig"
spec:
  extensions:
    helm:
      charts:
      - chartname: test
        name: test
        namespace: test
        version: 1.0.0
        values: |
          badyaml: true
          thisisnotavalidmapping: {{repl RandomString 10}}
`)},
			wantErr: "failed to parse helm chart values for addon test as yaml: error converting YAML to JSON: yaml: invalid map key: map[interface {}]interface {}{\"repl RandomString 10\":interface {}(nil)}",
		},
		{
			name: "good extension values",
			releaseData: map[string][]byte{
				"embedded-cluster-config.yaml": []byte(`
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
metadata:
  name: "testconfig"
spec:
  extensions:
    helm:
      charts:
      - chartname: test
        name: test
        namespace: test
        version: 1.0.0
        values: |
          goodyaml: true
`)},
			wantErr: "",
		},
		{
			name: "no extension values",
			releaseData: map[string][]byte{
				"embedded-cluster-config.yaml": []byte(`
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
metadata:
  name: "testconfig"
spec:
  extensions:
    helm:
      charts:
      - chartname: test
        name: test
        namespace: test
        version: 1.0.0
`)},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			req.NoError(SetReleaseDataForTests(tt.releaseData))

			err := ValidateECConfig()
			if tt.wantErr != "" {
				req.Error(err)
				req.Equal(tt.wantErr, err.Error())
			} else {
				req.NoError(err)
			}
		})
	}
}
