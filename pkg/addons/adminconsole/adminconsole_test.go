// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetEmbeddedRegistryCredentials(t *testing.T) {
	type args struct {
		kcli client.Client
	}
	tests := []struct {
		name    string
		args    args
		want    *EmbeddedRegistryCredentials
		wantErr bool
	}{
		{
			name: "happy path",
			args: args{
				kcli: fake.NewClientBuilder().WithObjects(
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "registry-creds", Namespace: "kotsadm"},
						Data: map[string][]byte{
							".dockerconfigjson": []byte(`{"auths":{"test-host":{"username":"embedded-cluster","password":"test-password"}}}`),
						},
					},
				).Build(),
			},
			want: &EmbeddedRegistryCredentials{
				Hostname: "test-host",
				Username: "embedded-cluster",
				Password: "test-password",
			},
			wantErr: false,
		},
		{
			name: "no embedded-cluster credentials",
			args: args{
				kcli: fake.NewClientBuilder().WithObjects(
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "registry-creds", Namespace: "kotsadm"},
						Data: map[string][]byte{
							".dockerconfigjson": []byte(`{"auths":{"test-host":{"username":"not-embedded-cluster","password":"test-password"}}}`),
						},
					},
				).Build(),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "no registry creds secret",
			args: args{
				kcli: fake.NewClientBuilder().Build(),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetEmbeddedRegistryCredentials(context.Background(), tt.args.kcli)
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
