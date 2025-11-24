package preflights

import (
	"context"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/require"
)

func TestPrepareHostPreflights(t *testing.T) {
	req := require.New(t)
	ctx := context.Background()

	opts := PrepareHostPreflightOptions{
		AdminConsolePort:        30000,
		LocalArtifactMirrorPort: 50000,
		DataDir:                 "/var/lib/embedded-cluster",
		K0sDataDir:              "/var/lib/k0s",
		OpenEBSDataDir:          "/var/openebs/local",
	}

	spec, err := PrepareHostPreflights(ctx, opts)
	req.NoError(err)
	req.NotNil(spec)

	// Verify we have collectors and analyzers
	req.NotEmpty(spec.Collectors, "Expected collectors to be present")
	req.NotEmpty(spec.Analyzers, "Expected analyzers to be present")

	// Check that resolv-conf.yaml preflights are included
	// 1. Check for the resolv.conf collector
	hasResolvConfCollector := false
	for _, c := range spec.Collectors {
		if c.HostRun != nil && c.HostRun.CollectorName == "resolv.conf" {
			hasResolvConfCollector = true
			req.Equal("sh", c.HostRun.Command)
			req.Equal([]string{"-c", "cat /etc/resolv.conf"}, c.HostRun.Args)
			break
		}
	}
	req.True(hasResolvConfCollector, "Expected resolv.conf collector from resolv-conf.yaml")

	// 2. Check for Resolver Configuration analyzer (local nameserver check)
	hasResolverConfigAnalyzer := false
	for _, a := range spec.Analyzers {
		if a.TextAnalyze != nil && a.TextAnalyze.CheckName == "Resolver Configuration" {
			hasResolverConfigAnalyzer = true
			req.Contains(a.TextAnalyze.RegexPattern, "localhost")
			req.Contains(a.TextAnalyze.RegexPattern, "::1") // IPv6 localhost
			break
		}
	}
	req.True(hasResolverConfigAnalyzer, "Expected Resolver Configuration analyzer from resolv-conf.yaml")

	// 3. Check for Nameserver Configuration analyzer (at least one nameserver check)
	hasNameserverConfigAnalyzer := false
	for _, a := range spec.Analyzers {
		if a.TextAnalyze != nil && a.TextAnalyze.CheckName == "Nameserver Configuration" {
			hasNameserverConfigAnalyzer = true
			// Verify it supports both IPv4 and IPv6
			req.Contains(a.TextAnalyze.RegexPattern, "\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}") // IPv4
			req.Contains(a.TextAnalyze.RegexPattern, "[0-9a-fA-F]*:[0-9a-fA-F:]+")                // IPv6
			break
		}
	}
	req.True(hasNameserverConfigAnalyzer, "Expected Nameserver Configuration analyzer from resolv-conf.yaml")

	// Check that host-preflight.yaml preflights are included
	// 4. Check for at least one collector from host-preflight.yaml (e.g., memory collector)
	hasMemoryCollector := false
	for _, c := range spec.Collectors {
		if c.Memory != nil {
			hasMemoryCollector = true
			break
		}
	}
	req.True(hasMemoryCollector, "Expected memory collector from host-preflight.yaml")

	// 5. Check for at least one analyzer from host-preflight.yaml (e.g., Memory analyzer)
	hasMemoryAnalyzer := false
	for _, a := range spec.Analyzers {
		if a.Memory != nil && a.Memory.CheckName == "Memory" {
			hasMemoryAnalyzer = true
			req.NotEmpty(a.Memory.Outcomes)
			break
		}
	}
	req.True(hasMemoryAnalyzer, "Expected Memory analyzer from host-preflight.yaml")
}

func TestPrepareHostPreflightsWithCustomSpec(t *testing.T) {
	req := require.New(t)
	ctx := context.Background()

	// Test that custom spec collectors/analyzers are preserved and merged
	customCollector := &troubleshootv1beta2.HostCollect{
		HostRun: &troubleshootv1beta2.HostRun{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: "custom-collector",
			},
			Command: "echo",
			Args:    []string{"test"},
		},
	}

	customAnalyzer := &troubleshootv1beta2.HostAnalyze{
		TextAnalyze: &troubleshootv1beta2.TextAnalyze{
			AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
				CheckName: "Custom Check",
			},
			FileName:     "custom.txt",
			RegexPattern: "test",
		},
	}

	opts := PrepareHostPreflightOptions{
		HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{
			Collectors: []*troubleshootv1beta2.HostCollect{customCollector},
			Analyzers:  []*troubleshootv1beta2.HostAnalyze{customAnalyzer},
		},
		AdminConsolePort:        30000,
		LocalArtifactMirrorPort: 50000,
		DataDir:                 "/var/lib/embedded-cluster",
		K0sDataDir:              "/var/lib/k0s",
		OpenEBSDataDir:          "/var/openebs/local",
	}

	spec, err := PrepareHostPreflights(ctx, opts)
	req.NoError(err)
	req.NotNil(spec)

	// Verify custom collector is preserved
	hasCustomCollector := false
	for _, c := range spec.Collectors {
		if c.HostRun != nil && c.HostRun.CollectorName == "custom-collector" {
			hasCustomCollector = true
			break
		}
	}
	req.True(hasCustomCollector, "Expected custom collector to be preserved")

	// Verify custom analyzer is preserved
	hasCustomAnalyzer := false
	for _, a := range spec.Analyzers {
		if a.TextAnalyze != nil && a.TextAnalyze.CheckName == "Custom Check" {
			hasCustomAnalyzer = true
			break
		}
	}
	req.True(hasCustomAnalyzer, "Expected custom analyzer to be preserved")

	// Verify cluster preflights are still added
	hasResolvConfCollector := false
	for _, c := range spec.Collectors {
		if c.HostRun != nil && c.HostRun.CollectorName == "resolv.conf" {
			hasResolvConfCollector = true
			break
		}
	}
	req.True(hasResolvConfCollector, "Expected resolv.conf collector to be added")
}
