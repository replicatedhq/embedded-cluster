package domains

import (
	"reflect"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

func TestGetDomains(t *testing.T) {
	type args struct {
		cfgspec *ecv1beta1.ConfigSpec
		rel     *release.ChannelRelease
	}
	tests := []struct {
		name string
		args args
		want ecv1beta1.Domains
	}{
		{
			name: "default domains nil",
			args: args{
				cfgspec: nil,
				rel:     nil,
			},
			want: ecv1beta1.Domains{
				ReplicatedAppDomain:      defaultReplicatedAppDomain,
				ProxyRegistryDomain:      defaultProxyRegistryDomain,
				ReplicatedRegistryDomain: defaultReplicatedRegistryDomain,
			},
		},
		{
			name: "default domains empty",
			args: args{
				cfgspec: &ecv1beta1.ConfigSpec{},
				rel:     &release.ChannelRelease{},
			},
			want: ecv1beta1.Domains{
				ReplicatedAppDomain:      defaultReplicatedAppDomain,
				ProxyRegistryDomain:      defaultProxyRegistryDomain,
				ReplicatedRegistryDomain: defaultReplicatedRegistryDomain,
			},
		},
		{
			name: "release domains",
			args: args{
				cfgspec: &ecv1beta1.ConfigSpec{},
				rel: &release.ChannelRelease{
					DefaultDomains: release.Domains{
						ReplicatedAppDomain:      "release.replicated.app",
						ProxyRegistryDomain:      "release.proxy.replicated.com",
						ReplicatedRegistryDomain: "release.registry.replicated.com",
					},
				},
			},
			want: ecv1beta1.Domains{
				ReplicatedAppDomain:      "release.replicated.app",
				ProxyRegistryDomain:      "release.proxy.replicated.com",
				ReplicatedRegistryDomain: "release.registry.replicated.com",
			},
		},
		{
			name: "config spec domains",
			args: args{
				cfgspec: &ecv1beta1.ConfigSpec{
					Domains: ecv1beta1.Domains{
						ReplicatedAppDomain:      "config.replicated.app",
						ProxyRegistryDomain:      "config.proxy.replicated.com",
						ReplicatedRegistryDomain: "config.registry.replicated.com",
					},
				},
				rel: &release.ChannelRelease{
					DefaultDomains: release.Domains{
						ReplicatedAppDomain:      "release.replicated.app",
						ProxyRegistryDomain:      "release.proxy.replicated.com",
						ReplicatedRegistryDomain: "release.registry.replicated.com",
					},
				},
			},
			want: ecv1beta1.Domains{
				ReplicatedAppDomain:      "config.replicated.app",
				ProxyRegistryDomain:      "config.proxy.replicated.com",
				ReplicatedRegistryDomain: "config.registry.replicated.com",
			},
		},
		{
			name: "a mix",
			args: args{
				cfgspec: &ecv1beta1.ConfigSpec{
					Domains: ecv1beta1.Domains{
						ReplicatedAppDomain: "config.replicated.app",
					},
				},
				rel: &release.ChannelRelease{
					DefaultDomains: release.Domains{
						ProxyRegistryDomain: "release.proxy.replicated.com",
					},
				},
			},
			want: ecv1beta1.Domains{
				ReplicatedAppDomain:      "config.replicated.app",
				ProxyRegistryDomain:      "release.proxy.replicated.com",
				ReplicatedRegistryDomain: defaultReplicatedRegistryDomain,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDomains(tt.args.cfgspec, tt.args.rel); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDomains() = %v, want %v", got, tt.want)
			}
		})
	}
}
