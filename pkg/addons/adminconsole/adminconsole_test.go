// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"context"
	"reflect"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetEmbeddedRegistryCredentials(t *testing.T) {
	type args struct {
		ctx  context.Context
		kcli client.Client
	}
	tests := []struct {
		name    string
		args    args
		want    *EmbeddedRegistryCredentials
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetEmbeddedRegistryCredentials(tt.args.ctx, tt.args.kcli)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEmbeddedRegistryCredentials() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetEmbeddedRegistryCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}
