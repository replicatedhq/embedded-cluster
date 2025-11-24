package preflight

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	troubleshootanalyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type PrepareHostPreflightOptions struct {
	ReplicatedAppURL       string
	ProxyRegistryURL       string
	HostPreflightSpec      *troubleshootv1beta2.HostPreflightSpec
	EmbeddedClusterConfig  *ecv1beta1.Config
	TCPConnectionsRequired []string
	IsAirgap               bool
	IsJoin                 bool
	IsUI                   bool
	AirgapInfo             *kotsv1beta1.Airgap
	EmbeddedAssetsSize     int64
}

type RunHostPreflightOptions struct {
	HostPreflightSpec *troubleshootv1beta2.HostPreflightSpec
}

func (m *hostPreflightManager) PrepareHostPreflights(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, error) {
	// Get node IP
	nodeIP, err := m.netUtils.FirstValidAddress(rc.NetworkInterface())
	if err != nil {
		return nil, fmt.Errorf("determine node ip: %w", err)
	}

	prepareOpts := buildPrepareHostPreflightOptions(rc, opts, nodeIP)

	// Use the shared Prepare function to prepare host preflights
	hpf, err := preflights.PrepareHostPreflights(ctx, prepareOpts)
	if err != nil {
		return nil, fmt.Errorf("prepare host preflights: %w", err)
	}

	return hpf, nil
}

// buildPrepareHostPreflightOptions (Hop): builds the options for the preflights.PrepareHostPreflights function
func buildPrepareHostPreflightOptions(rc runtimeconfig.RuntimeConfig, opts PrepareHostPreflightOptions, nodeIP string) preflights.PrepareHostPreflightOptions {
	// Calculate airgap storage space requirement (2x uncompressed size for controller nodes)
	var controllerAirgapStorageSpace string
	if opts.AirgapInfo != nil {
		controllerAirgapStorageSpace = preflights.CalculateAirgapStorageSpace(preflights.AirgapStorageSpaceCalcArgs{
			UncompressedSize:   opts.AirgapInfo.Spec.UncompressedSize,
			EmbeddedAssetsSize: opts.EmbeddedAssetsSize,
			K0sImageSize:       opts.AirgapInfo.Spec.UncompressedSize,
			IsController:       true,
		})
	}

	prepareOpts := preflights.PrepareHostPreflightOptions{
		HostPreflightSpec:            opts.HostPreflightSpec,
		ReplicatedAppURL:             opts.ReplicatedAppURL,
		ProxyRegistryURL:             opts.ProxyRegistryURL,
		AdminConsolePort:             rc.AdminConsolePort(),
		LocalArtifactMirrorPort:      rc.LocalArtifactMirrorPort(),
		DataDir:                      rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:                   rc.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:               rc.EmbeddedClusterOpenEBSLocalSubDir(),
		Proxy:                        rc.ProxySpec(),
		PodCIDR:                      rc.PodCIDR(),
		ServiceCIDR:                  rc.ServiceCIDR(),
		NodeIP:                       nodeIP,
		IsAirgap:                     opts.IsAirgap,
		TCPConnectionsRequired:       opts.TCPConnectionsRequired,
		IsJoin:                       opts.IsJoin,
		IsUI:                         opts.IsUI,
		IsV3:                         true,
		ControllerAirgapStorageSpace: controllerAirgapStorageSpace,
	}
	if cidr := rc.GlobalCIDR(); cidr != "" {
		prepareOpts.GlobalCIDR = &cidr
	}

	return prepareOpts
}

func (m *hostPreflightManager) RunHostPreflights(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts RunHostPreflightOptions) (*types.PreflightsOutput, error) {
	titles, err := m.getTitles(opts.HostPreflightSpec)
	if err != nil {
		return nil, fmt.Errorf("get titles: %w", err)
	}

	if err := m.hostPreflightStore.SetTitles(titles); err != nil {
		return nil, fmt.Errorf("set titles: %w", err)
	}

	if err := m.hostPreflightStore.SetOutput(nil); err != nil {
		return nil, fmt.Errorf("reset output: %w", err)
	}

	// Run the preflights using the shared core function
	runOpts := preflights.RunOptions{
		PreflightBinaryPath: rc.PathToEmbeddedClusterBinary("kubectl-preflight"),
		ProxySpec:           rc.ProxySpec(),
		ExtraPaths:          []string{rc.EmbeddedClusterBinsSubDir()},
	}

	output, stderr, err := m.runner.RunHostPreflights(ctx, opts.HostPreflightSpec, runOpts)
	if err != nil {
		if stderr != "" {
			return nil, fmt.Errorf("host preflights failed to run: %w (stderr: %s)", err, stderr)
		}
		return nil, fmt.Errorf("host preflights failed to run: %w", err)
	}

	// Set final status based on results
	if err := m.hostPreflightStore.SetOutput(output); err != nil {
		return nil, fmt.Errorf("set output: %w", err)
	}

	if err := preflights.SaveToDisk(output, rc.PathToEmbeddedClusterSupportFile("host-preflight-results.json")); err != nil {
		m.logger.WithError(err).Warn("save preflights output")
	}

	if err := preflights.CopyBundleTo(rc.PathToEmbeddedClusterSupportFile("preflight-bundle.tar.gz")); err != nil {
		m.logger.WithError(err).Warn("copy preflight bundle to embedded-cluster support dir")
	}

	return output, nil
}

func (m *hostPreflightManager) GetHostPreflightStatus(ctx context.Context) (types.Status, error) {
	return m.hostPreflightStore.GetStatus()
}

func (m *hostPreflightManager) GetHostPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error) {
	return m.hostPreflightStore.GetOutput()
}

func (m *hostPreflightManager) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	return m.hostPreflightStore.GetTitles()
}

func (m *hostPreflightManager) ClearHostPreflightResults(ctx context.Context) error {
	return m.hostPreflightStore.Clear()
}

func (m *hostPreflightManager) getTitles(hpf *troubleshootv1beta2.HostPreflightSpec) ([]string, error) {
	if hpf == nil || hpf.Analyzers == nil {
		return nil, nil
	}

	titles := []string{}
	for _, a := range hpf.Analyzers {
		analyzer, ok := troubleshootanalyze.GetHostAnalyzer(a)
		if !ok {
			continue
		}
		excluded, err := analyzer.IsExcluded()
		if err != nil {
			return nil, fmt.Errorf("check if analyzer is excluded: %w", err)
		}
		if !excluded {
			titles = append(titles, analyzer.Title())
		}
	}

	return titles, nil
}
