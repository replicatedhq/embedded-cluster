package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_splitManifests(t *testing.T) {
	tests := []struct {
		name      string
		yamlInput string
		want      [][]byte
		wantErr   bool
	}{
		{
			name: "multiple YAML documents",
			yamlInput: `apiVersion: v1
kind: Service
metadata:
  name: test-service
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment`,
			want: [][]byte{
				[]byte("apiVersion: v1\nkind: Service\nmetadata:\n  name: test-service"),
				[]byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test-deployment"),
			},
			wantErr: false,
		},
		{
			name: "single YAML document",
			yamlInput: `apiVersion: v1
kind: Service
metadata:
  name: test-service`,
			want: [][]byte{
				[]byte("apiVersion: v1\nkind: Service\nmetadata:\n  name: test-service"),
			},
			wantErr: false,
		},
		{
			name:      "empty input",
			yamlInput: "",
			want:      [][]byte{},
			wantErr:   false,
		},
		{
			name: "documents with whitespace around separators",
			yamlInput: `apiVersion: v1
kind: ConfigMap
metadata:
  name: config1

  ---  

apiVersion: v1
kind: ConfigMap
metadata:
  name: config2`,
			want: [][]byte{
				[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: config1"),
				[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: config2"),
			},
			wantErr: false,
		},
		{
			name: "document starting with separator",
			yamlInput: `---
apiVersion: v1
kind: Service
metadata:
  name: test-service
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment`,
			want: [][]byte{
				[]byte("apiVersion: v1\nkind: Service\nmetadata:\n  name: test-service"),
				[]byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test-deployment"),
			},
			wantErr: false,
		},
		{
			name: "yaml content containing triple dash",
			yamlInput: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  message: "This contains --- in the middle but should not split here"
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret`,
			want: [][]byte{
				[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test-config\ndata:\n  message: \"This contains --- in the middle but should not split here\""),
				[]byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: test-secret"),
			},
			wantErr: false,
		},
		{
			name: "complex whitespace variations",
			yamlInput: `  apiVersion: v1
kind: ConfigMap
metadata:
  name: config1  

   ---   
   
apiVersion: v1
kind: ConfigMap  
metadata:
  name: config2
  
	---	
	
apiVersion: v1
kind: ConfigMap
metadata:
  name: config3   `,
			want: [][]byte{
				[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: config1"),
				[]byte("apiVersion: v1\nkind: ConfigMap  \nmetadata:\n  name: config2"),
				[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: config3"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitManifests(tt.yamlInput)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, len(tt.want), len(got))
			for i, expected := range tt.want {
				assert.Equal(t, string(expected), string(got[i]))
			}
		})
	}
}
