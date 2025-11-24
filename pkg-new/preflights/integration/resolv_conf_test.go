package preflights

import (
	"context"
	"encoding/base64"
	"os"
	"testing"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
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
		name             string
		fileContent      string
		findAnalyzerFn   func(a *troubleshootv1beta2.HostAnalyze) bool
		modifyAnalyzerFn func(a *troubleshootv1beta2.HostAnalyze, fileName string)
		expectOutput     *apitypes.PreflightsOutput
	}{
		{
			name:        "valid IPv4 nameserver passes",
			fileContent: "nameserver 8.8.8.8\n",
			findAnalyzerFn: func(a *troubleshootv1beta2.HostAnalyze) bool {
				return a.TextAnalyze != nil && a.TextAnalyze.CheckName == "Resolver Configuration"
			},
			modifyAnalyzerFn: func(a *troubleshootv1beta2.HostAnalyze, fileName string) {
				a.TextAnalyze.FileName = fileName
			},
			expectOutput: &apitypes.PreflightsOutput{
				Pass: []apitypes.PreflightsRecord{
					{
						Title:   "Resolver Configuration",
						Message: "No local nameserver entries detected in resolv.conf",
					},
				},
				Fail: nil,
				Warn: nil,
			},
		},
		// {
		// 	name:              "valid IPv6 nameserver passes",
		// 	resolvConfContent: "nameserver 2001:4860:4860::8888\n",
		// 	expectPass:        true,
		// 	expectFail:        false,
		// },
		// {
		// 	name:              "mixed IPv4 and IPv6 nameservers pass",
		// 	resolvConfContent: "nameserver 8.8.8.8\nnameserver 2001:4860:4860::8888\n",
		// 	expectPass:        true,
		// 	expectFail:        false,
		// },
		// {
		// 	name:              "no nameservers fails",
		// 	resolvConfContent: "search example.com\n",
		// 	expectPass:        false,
		// 	expectFail:        true,
		// },
		// {
		// 	name:              "IPv4 localhost fails",
		// 	resolvConfContent: "nameserver 127.0.0.1\n",
		// 	expectPass:        false,
		// 	expectFail:        true,
		// },
		// {
		// 	name:              "IPv6 localhost fails",
		// 	resolvConfContent: "nameserver ::1\n",
		// 	expectPass:        false,
		// 	expectFail:        true,
		// },
		// {
		// 	name:              "IPv4-mapped IPv6 localhost fails",
		// 	resolvConfContent: "nameserver ::ffff:127.0.0.1\n",
		// 	expectPass:        false,
		// 	expectFail:        true,
		// },
		// {
		// 	name:              "mixed public and localhost fails",
		// 	resolvConfContent: "nameserver 8.8.8.8\nnameserver 127.0.0.1\n",
		// 	expectPass:        false,
		// 	expectFail:        true,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			ctx := context.Background()

			hpfs, err := preflights.GetClusterHostPreflights(ctx, types.HostPreflightTemplateData{})
			req.NoError(err)
			req.NotEmpty(hpfs)

			// Find the resolv-conf preflight spec
			var analyzer *troubleshootv1beta2.HostAnalyze
			for _, hpf := range hpfs {
				if a := findAnalyzer(hpf.Spec.Analyzers, tt.findAnalyzerFn); a != nil {
					analyzer = a
					break
				}
			}
			req.NotNil(analyzer, "Expected to find analyzer")

			tt.modifyAnalyzerFn(analyzer, "host-collectors/run-host/test.txt")

			base64Content := base64.StdEncoding.EncodeToString([]byte(tt.fileContent))

			// Run the preflight binary
			runner := preflights.New()
			opts := preflights.RunOptions{
				PreflightBinaryPath: preflightBinary,
			}

			spec := &troubleshootv1beta2.HostPreflightSpec{
				Collectors: []*troubleshootv1beta2.HostCollect{
					{
						HostRun: &troubleshootv1beta2.HostRun{
							HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
								CollectorName: "test",
							},
							Command: "sh",
							Args:    []string{"-c", "echo -n '" + base64Content + "' | base64 -d"},
						},
					},
				},
				Analyzers: []*troubleshootv1beta2.HostAnalyze{analyzer},
			}

			t.Chdir(t.TempDir()) // Change to temp dir for preflight bundle cleanup

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
			req.Equal(tt.expectOutput, output)
		})
	}
}

func findAnalyzer(analyzers []*troubleshootv1beta2.HostAnalyze, fn func(*troubleshootv1beta2.HostAnalyze) bool) *troubleshootv1beta2.HostAnalyze {
	for _, a := range analyzers {
		if fn(a) {
			return a
		}
	}
	return nil
}
