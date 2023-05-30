package controller

import (
	"os"
	"reflect"
	"testing"
)

func Test_mergeK0sConfigFiles(t *testing.T) {
	tests := []struct {
		name        string
		cfgFileData string
		want        []byte
		wantErr     bool
	}{
		{
			name: "basic",
			cfgFileData: `spec:
  api:
    externalAddress: 10.0.0.1
`,
			want: []byte(`apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
spec:
  api:
    externalAddress: 10.0.0.1
    extraArgs:
      service-node-port-range: 80-60000
  extensions:
    storage:
      create_default_storage_class: true
      type: openebs_local_storage
  network:
    provider: calico
`,
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.CreateTemp("", "k0s-controller-test")
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(f.Name(), []byte(tt.cfgFileData), 0644)
			if err != nil {
				t.Fatal(err)
			}
			got, err := mergeK0sConfigFiles(f.Name())
			if (err != nil) != tt.wantErr {
				t.Errorf("mergeK0sConfigFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeK0sConfigFiles() = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}
