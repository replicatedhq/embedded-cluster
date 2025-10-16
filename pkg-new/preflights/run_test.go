// Package preflights manages running host preflights on remote hosts.
package preflights

import (
	"strings"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
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
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_pathEnv(t *testing.T) {
	rc := runtimeconfig.New(nil)
	rc.SetDataDir(t.TempDir())
	binDir := rc.EmbeddedClusterBinsSubDir()

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
					"PATH=/usr/bin:/usr/local/bin",
				},
			},
			want: []string{
				"TEST=test",
				"PATH=/usr/bin:/usr/local/bin:" + binDir,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathEnv(tt.args.env, []string{binDir})
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_preparePreflightEnv(t *testing.T) {
	rc := runtimeconfig.New(nil)
	rc.SetDataDir(t.TempDir())
	binDir := rc.EmbeddedClusterBinsSubDir()

	type args struct {
		env  []string
		opts RunOptions
	}
	tests := []struct {
		name                   string
		args                   args
		wantProxySet           bool
		wantPathIncludesBinDir bool
	}{
		{
			name: "empty env with no options",
			args: args{
				env:  []string{},
				opts: RunOptions{},
			},
			wantProxySet:           false,
			wantPathIncludesBinDir: false,
		},
		{
			name: "env with proxy settings",
			args: args{
				env: []string{"TEST=test"},
				opts: RunOptions{
					ProxySpec: &ecv1beta1.ProxySpec{
						HTTPProxy:  "http://proxy:8080",
						HTTPSProxy: "https://proxy:8080",
						NoProxy:    "localhost",
					},
				},
			},
			wantProxySet:           true,
			wantPathIncludesBinDir: false,
		},
		{
			name: "env with extra paths",
			args: args{
				env: []string{"TEST=test", "PATH=/usr/bin"},
				opts: RunOptions{
					ExtraPaths: []string{binDir},
				},
			},
			wantProxySet:           false,
			wantPathIncludesBinDir: true,
		},
		{
			name: "env with all options",
			args: args{
				env: []string{"TEST=test", "PATH=/usr/bin"},
				opts: RunOptions{
					ProxySpec: &ecv1beta1.ProxySpec{
						HTTPProxy:  "http://proxy:8080",
						HTTPSProxy: "https://proxy:8080",
						NoProxy:    "localhost",
					},
					ExtraPaths: []string{binDir},
				},
			},
			wantProxySet:           true,
			wantPathIncludesBinDir: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preparePreflightEnv(tt.args.env, tt.args.opts)

			// Convert to map for easier checking
			gotMap := make(map[string]string)
			for _, e := range got {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) == 2 {
					gotMap[parts[0]] = parts[1]
				}
			}

			// PREFLIGHT_AUTO_UPDATE should always be disabled
			assert.Equal(t, "false", gotMap["PREFLIGHT_AUTO_UPDATE"], "PREFLIGHT_AUTO_UPDATE must always be set to false")

			// Verify proxy settings if expected
			if tt.wantProxySet {
				assert.Contains(t, gotMap, "HTTP_PROXY", "HTTP_PROXY should be set")
				assert.Contains(t, gotMap, "HTTPS_PROXY", "HTTPS_PROXY should be set")
				assert.Contains(t, gotMap, "NO_PROXY", "NO_PROXY should be set")
				assert.Equal(t, "http://proxy:8080", gotMap["HTTP_PROXY"])
				assert.Equal(t, "https://proxy:8080", gotMap["HTTPS_PROXY"])
				assert.Equal(t, "localhost", gotMap["NO_PROXY"])
			}

			// Verify PATH includes bin directory if expected
			if tt.wantPathIncludesBinDir {
				assert.Contains(t, gotMap, "PATH", "PATH should be set")
				assert.Contains(t, gotMap["PATH"], binDir, "PATH should include bin directory")
			}
		})
	}
}
