package controllers

import (
	"reflect"
	"testing"

	"github.com/k0sproject/version"
)

func Test_k8sServerVersionFromK0sVersion(t *testing.T) {
	tests := []struct {
		k0sVersion string
		want       *version.Version
		wantErr    bool
	}{
		{
			k0sVersion: "v1.29.5+k0s.1",
			want:       version.MustParse("1.29.5+k0s"),
		},
		{
			k0sVersion: "v1.29.5+k0s",
			want:       version.MustParse("1.29.5+k0s"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.k0sVersion, func(t *testing.T) {
			got, err := k8sServerVersionFromK0sVersion(tt.k0sVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("k8sServerVersionFromK0sVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("k8sServerVersionFromK0sVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
