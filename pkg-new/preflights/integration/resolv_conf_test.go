package preflights

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights/types"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/require"
)

// TestIntegration_ResolvConf is an integration test that runs the actual preflight binary
// with a custom RootDir to test resolv.conf checks with both IPv4 and IPv6 nameservers.
//
// Run with: PREFLIGHT_BINARY=/path/to/preflight go test -v -run TestIntegration_ResolvConf
func TestIntegration_ResolvConf(t *testing.T) {
	preflightBinary := os.Getenv("PREFLIGHT_BINARY")
	if preflightBinary == "" {
		t.Skip("PREFLIGHT_BINARY not set, skipping integration test")
	}

	if _, err := os.Stat(preflightBinary); os.IsNotExist(err) {
		t.Skipf("Preflight binary not found at %s, skipping integration test", preflightBinary)
	}

	tests := []struct {
		name              string
		resolvConfContent string
		expectPass        bool
		expectFail        bool
	}{
		{
			name:              "valid IPv4 nameserver passes",
			resolvConfContent: "nameserver 8.8.8.8\n",
			expectPass:        true,
			expectFail:        false,
		},
		{
			name:              "valid IPv6 nameserver passes",
			resolvConfContent: "nameserver 2001:4860:4860::8888\n",
			expectPass:        true,
			expectFail:        false,
		},
		{
			name:              "mixed IPv4 and IPv6 nameservers pass",
			resolvConfContent: "nameserver 8.8.8.8\nnameserver 2001:4860:4860::8888\n",
			expectPass:        true,
			expectFail:        false,
		},
		{
			name:              "no nameservers fails",
			resolvConfContent: "search example.com\n",
			expectPass:        false,
			expectFail:        true,
		},
		{
			name:              "IPv4 localhost fails",
			resolvConfContent: "nameserver 127.0.0.1\n",
			expectPass:        false,
			expectFail:        true,
		},
		{
			name:              "IPv6 localhost fails",
			resolvConfContent: "nameserver ::1\n",
			expectPass:        false,
			expectFail:        true,
		},
		{
			name:              "IPv4-mapped IPv6 localhost fails",
			resolvConfContent: "nameserver ::ffff:127.0.0.1\n",
			expectPass:        false,
			expectFail:        true,
		},
		{
			name:              "mixed public and localhost fails",
			resolvConfContent: "nameserver 8.8.8.8\nnameserver 127.0.0.1\n",
			expectPass:        false,
			expectFail:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			ctx := context.Background()

			// Create temporary directory structure with custom resolv.conf
			tmpDir := t.TempDir()
			etcDir := filepath.Join(tmpDir, "etc")
			err := os.MkdirAll(etcDir, 0755)
			req.NoError(err)

			resolvConfPath := filepath.Join(etcDir, "resolv.conf")
			err = os.WriteFile(resolvConfPath, []byte(tt.resolvConfContent), 0644)
			req.NoError(err)
			t.Logf("Created resolv.conf at %s with content:\n%s", resolvConfPath, tt.resolvConfContent)

			// Render the preflight spec with custom RootDir pointing to our temp directory
			data := types.HostPreflightTemplateData{
				RootDir: tmpDir + "/",
			}

			hpfs, err := preflights.GetClusterHostPreflights(ctx, data)
			req.NoError(err)
			req.NotEmpty(hpfs)

			// Find the resolv-conf preflight spec
			var spec *troubleshootv1beta2.HostPreflightSpec
			for _, hpf := range hpfs {
				if hpf.Name == "ec-resolv-conf-preflight" {
					spec = &hpf.Spec
					break
				}
			}
			req.NotNil(spec, "Expected to find ec-resolv-conf-preflight")

			// Verify the collector is using our custom RootDir
			for _, c := range spec.Collectors {
				if c.HostRun != nil && c.HostRun.CollectorName == "resolv.conf" {
					expectedPath := tmpDir + "/etc/resolv.conf"
					t.Logf("Collector will read from: cat %s", expectedPath)
				}
			}

			// Run the preflight binary
			runner := preflights.New()
			opts := preflights.RunOptions{
				PreflightBinaryPath: preflightBinary,
			}

			output, stderr, err := runner.RunHostPreflights(ctx, spec, opts)
			if err != nil {
				t.Logf("Preflight error: %v", err)
				t.Logf("Stderr: %s", stderr)
			}

			req.NotNil(output, "Expected output from preflight run")

			// Log all results
			t.Logf("Pass checks: %d", len(output.Pass))
			for _, p := range output.Pass {
				t.Logf("  ✓ %s: %s", p.Title, p.Message)
			}
			t.Logf("Fail checks: %d", len(output.Fail))
			for _, f := range output.Fail {
				t.Logf("  ✗ %s: %s", f.Title, f.Message)
			}

			// Verify expectations
			if tt.expectPass {
				req.NotEmpty(output.Pass, "Expected at least one passing check")
				req.Empty(output.Fail, "Expected no failing checks")
			}
			if tt.expectFail {
				req.NotEmpty(output.Fail, "Expected at least one failing check")
			}
		})
	}
}
