// Package preflights manages running host preflights on remote hosts.
package preflights

import (
	"strings"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
)

func Test_proxyEnv(t *testing.T) {
	type args struct {
		proxy *ecv1beta1.ProxySpec
	}
	tests := []struct {
		name           string
		env            map[string]string
		args           args
		wantHTTPProxy  string
		wantHTTPSProxy string
		wantNoProxy    string
	}{
		{
			name: "no proxy nil",
			args: args{
				proxy: nil,
			},
			wantHTTPProxy:  "",
			wantHTTPSProxy: "",
			wantNoProxy:    "",
		},
		{
			name: "no proxy nil",
			args: args{
				proxy: &ecv1beta1.ProxySpec{},
			},
			wantHTTPProxy:  "",
			wantHTTPSProxy: "",
			wantNoProxy:    "",
		},
		{
			name: "no proxy unset env",
			env: map[string]string{
				"HTTP_PROXY":  "http://proxy:8080",
				"HTTPS_PROXY": "https://proxy:8080",
				"NO_PROXY":    "localhost",
				"http_proxy":  "http://proxy:8080",
				"https_proxy": "https://proxy:8080",
				"no_proxy":    "localhost",
			},
			args: args{
				proxy: &ecv1beta1.ProxySpec{},
			},
			wantHTTPProxy:  "",
			wantHTTPSProxy: "",
			wantNoProxy:    "",
		},
		{
			name: "no proxy set env",
			args: args{
				proxy: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy:8080",
					HTTPSProxy: "https://proxy:8080",
					NoProxy:    "localhost",
				},
			},
			wantHTTPProxy:  "http://proxy:8080",
			wantHTTPSProxy: "https://proxy:8080",
			wantNoProxy:    "localhost",
		},
		{
			name: "proxy override env",
			env: map[string]string{
				"HTTP_PROXY":  "http://proxy:8080",
				"HTTPS_PROXY": "https://proxy:8080",
				"NO_PROXY":    "localhost",
				"http_proxy":  "http://proxy:8080",
				"https_proxy": "https://proxy:8080",
				"no_proxy":    "localhost",
			},
			args: args{
				proxy: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy2:8080",
					HTTPSProxy: "https://proxy2:8080",
					NoProxy:    "localhost2",
				},
			},
			wantHTTPProxy:  "http://proxy2:8080",
			wantHTTPSProxy: "https://proxy2:8080",
			wantNoProxy:    "localhost2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Unset proxy environment variables
			for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"} {
				t.Setenv(k, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got := proxyEnv(tt.args.proxy)
			gotMap := make(map[string]string)
			for _, e := range got {
				parts := strings.SplitN(e, "=", 2)
				gotMap[parts[0]] = parts[1]
			}
			assert.Equal(t, tt.wantHTTPProxy, gotMap["HTTP_PROXY"])
			assert.Equal(t, tt.wantHTTPSProxy, gotMap["HTTPS_PROXY"])
			assert.Equal(t, tt.wantNoProxy, gotMap["NO_PROXY"])
			assert.Empty(t, gotMap["http_proxy"])
			assert.Empty(t, gotMap["https_proxy"])
			assert.Empty(t, gotMap["no_proxy"])
		})
	}
}
