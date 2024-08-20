// Package preflights manages running host preflights on remote hosts.
package preflights

import (
	"os"
	"strings"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/stretchr/testify/assert"
)

func Test_proxyEnv(t *testing.T) {
	type args struct {
		env   []string
		proxy *ecv1beta1.ProxySpec
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "no proxy nil",
			args: args{
				env: []string{
					"TEST=test",
				},
				proxy: nil,
			},
			want: []string{
				"TEST=test",
			},
		},
		{
			name: "no proxy empty",
			args: args{
				env: []string{
					"TEST=test",
				},
				proxy: &ecv1beta1.ProxySpec{},
			},
			want: []string{
				"TEST=test",
				"HTTP_PROXY=",
				"HTTPS_PROXY=",
				"NO_PROXY=",
			},
		},
		{
			name: "no proxy unset env",
			args: args{
				env: []string{
					"TEST=test",
					"HTTP_PROXY=http://proxy:8080",
					"HTTPS_PROXY=https://proxy:8080",
					"NO_PROXY=localhost",
					"http_proxy=http://proxy:8080",
					"https_proxy=https://proxy:8080",
					"no_proxy=localhost",
				},
				proxy: &ecv1beta1.ProxySpec{},
			},
			want: []string{
				"TEST=test",
				"HTTP_PROXY=",
				"HTTPS_PROXY=",
				"NO_PROXY=",
			},
		},
		{
			name: "no proxy set env",
			args: args{
				env: []string{
					"TEST=test",
				},
				proxy: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy:8080",
					HTTPSProxy: "https://proxy:8080",
					NoProxy:    "localhost",
				},
			},
			want: []string{
				"TEST=test",
				"HTTP_PROXY=http://proxy:8080",
				"HTTPS_PROXY=https://proxy:8080",
				"NO_PROXY=localhost",
			},
		},
		{
			name: "proxy override env",
			args: args{
				env: []string{
					"TEST=test",
					"HTTP_PROXY=http://proxy:8080",
					"HTTPS_PROXY=https://proxy:8080",
					"NO_PROXY=localhost",
					"http_proxy=http://proxy:8080",
					"https_proxy=https://proxy:8080",
					"no_proxy=localhost",
				},
				proxy: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy2:8080",
					HTTPSProxy: "https://proxy2:8080",
					NoProxy:    "localhost2",
				},
			},
			want: []string{
				"TEST=test",
				"HTTP_PROXY=http://proxy2:8080",
				"HTTPS_PROXY=https://proxy2:8080",
				"NO_PROXY=localhost2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := proxyEnv(tt.args.env, tt.args.proxy)
			gotMap := make(map[string]string)
			for _, e := range got {
				parts := strings.SplitN(e, "=", 2)
				gotMap[parts[0]] = parts[1]
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_pathEnv(t *testing.T) {
	dir, err := os.MkdirTemp("", "embedded-cluster")
	if err != nil {
		t.Fatal(err)
	}

	oDefaultProvider := defaults.DefaultProvider
	defaults.DefaultProvider = defaults.NewProvider(dir)
	t.Cleanup(func() {
		defaults.DefaultProvider = oDefaultProvider
	})

	binDir := defaults.DefaultProvider.EmbeddedClusterBinsSubDir()

	type args struct {
		env []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "path empty",
			args: args{
				env: []string{
					"TEST=test",
				},
			},
			want: []string{
				"TEST=test",
				"PATH=" + binDir,
			},
		},
		{
			name: "path override",
			args: args{
				env: []string{
					"TEST=test",
					"PATH=/usr/bin,/usr/local/bin",
				},
			},
			want: []string{
				"TEST=test",
				"PATH=/usr/bin,/usr/local/bin," + binDir,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathEnv(tt.args.env)
			gotMap := make(map[string]string)
			for _, e := range got {
				parts := strings.SplitN(e, "=", 2)
				gotMap[parts[0]] = parts[1]
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
