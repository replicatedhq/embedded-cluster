package configutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/stretchr/testify/require"
)

func TestValidateKotsConfigValues(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		wantErr  bool
	}{
		{
			name: "valid",
			contents: `
apiVersion: kots.io/v1beta1
kind: ConfigValues
spec:
  configValues:
    - name: hostname
      value: 127.0.0.1
    - name: pw
      value: password
`,
			wantErr: false,
		},
		{
			name: "invalid - different kind",
			contents: `
apiVersion: kots.io/v1beta1
kind: Config`,
			wantErr: true,
		},
		{
			name:     "invalid - empty",
			contents: ``,
			wantErr:  true,
		},
		{
			name: "invalid - no kind",
			contents: `
apiVersion: kots.io/v1beta1`,
			wantErr: true,
		},
		{
			name: "invalid - different apiVersion",
			contents: `
apiVersion: kots.io/v1beta2
kind: ConfigValues
`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			tmp, err := helpers.MkdirTemp(os.TempDir(), "config-values-*")
			req.NoError(err)
			filename := filepath.Join(tmp, "config-values.yaml")
			err = helpers.WriteFile(filename, []byte(tt.contents), 0644)
			req.NoError(err)
			defer helpers.RemoveAll(tmp)

			gotErr := ValidateKotsConfigValues(filename)
			if tt.wantErr {
				req.Error(gotErr)
				return
			}
			req.NoError(gotErr)
		})
	}
}
