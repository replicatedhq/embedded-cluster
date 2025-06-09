package preflight

import (
	"context"
	"fmt"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootanalyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type PrepareHostPreflightOptions struct {
	RuntimeConfig          runtimeconfig.RuntimeConfig
	InstallationConfig     *types.InstallationConfig
	ReplicatedAppURL       string
	ProxyRegistryURL       string
	HostPreflightSpec      *troubleshootv1beta2.HostPreflightSpec
	EmbeddedClusterConfig  *ecv1beta1.Config
	TCPConnectionsRequired []string
	IsAirgap               bool
	IsJoin                 bool
}

type RunHostPreflightOptions struct {
	RuntimeConfig     runtimeconfig.RuntimeConfig
	HostPreflightSpec *troubleshootv1beta2.HostPreflightSpec
	Proxy             *ecv1beta1.ProxySpec
}

func (m *hostPreflightManager) PrepareHostPreflights(ctx context.Context, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, *ecv1beta1.ProxySpec, error) {
	hpf, proxy, err := m.prepareHostPreflights(ctx, opts)
	if err != nil {
		return nil, nil, err
	}
	return hpf, proxy, nil
}

func (m *hostPreflightManager) RunHostPreflights(ctx context.Context, opts RunHostPreflightOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.hostPreflightStore.IsRunning() {
		return fmt.Errorf("host preflights are already running")
	}

	if err := m.setRunningStatus(opts.HostPreflightSpec); err != nil {
		return fmt.Errorf("set running status: %w", err)
	}

	// Run preflights in background
	go m.runHostPreflights(context.Background(), opts)

	return nil
}

func (m *hostPreflightManager) GetHostPreflightStatus(ctx context.Context) (*types.Status, error) {
	return m.hostPreflightStore.GetStatus()
}

func (m *hostPreflightManager) GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightOutput, error) {
	return m.hostPreflightStore.GetOutput()
}

func (m *hostPreflightManager) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	return m.hostPreflightStore.GetTitles()
}

func (m *hostPreflightManager) prepareHostPreflights(ctx context.Context, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, *ecv1beta1.ProxySpec, error) {
	// Use provided installation config
	config := opts.InstallationConfig
	if config == nil {
		return nil, nil, fmt.Errorf("installation config is required")
	}

	// Use the provided runtime config
	rc := opts.RuntimeConfig
	if rc == nil {
		return nil, nil, fmt.Errorf("runtime config is required")
	}

	// Get node IP
	nodeIP, err := netutils.FirstValidAddress(config.NetworkInterface)
	if err != nil {
		return nil, nil, fmt.Errorf("determine node ip: %w", err)
	}

	// Build proxy spec
	var proxy *ecv1beta1.ProxySpec
	if config.HTTPProxy != "" || config.HTTPSProxy != "" || config.NoProxy != "" {
		proxy = &ecv1beta1.ProxySpec{
			HTTPProxy:  config.HTTPProxy,
			HTTPSProxy: config.HTTPSProxy,
			NoProxy:    config.NoProxy,
		}
	}

	var globalCIDR *string
	if config.GlobalCIDR != "" {
		globalCIDR = &config.GlobalCIDR
	}

	// Use the shared Prepare function to prepare host preflights
	hpf, err := preflights.Prepare(ctx, preflights.PrepareOptions{
		HostPreflightSpec:       opts.HostPreflightSpec,
		ReplicatedAppURL:        opts.ReplicatedAppURL,
		ProxyRegistryURL:        opts.ProxyRegistryURL,
		AdminConsolePort:        opts.InstallationConfig.AdminConsolePort,
		LocalArtifactMirrorPort: opts.InstallationConfig.LocalArtifactMirrorPort,
		DataDir:                 opts.InstallationConfig.DataDirectory,
		K0sDataDir:              rc.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:          rc.EmbeddedClusterOpenEBSLocalSubDir(),
		Proxy:                   proxy,
		PodCIDR:                 config.PodCIDR,
		ServiceCIDR:             config.ServiceCIDR,
		GlobalCIDR:              globalCIDR,
		NodeIP:                  nodeIP,
		IsAirgap:                opts.IsAirgap,
		TCPConnectionsRequired:  opts.TCPConnectionsRequired,
		IsJoin:                  opts.IsJoin,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("prepare host preflights: %w", err)
	}

	return hpf, proxy, nil
}

func (m *hostPreflightManager) runHostPreflights(ctx context.Context, opts RunHostPreflightOptions) {
	defer func() {
		if r := recover(); r != nil {
			if err := m.setFailedStatus(fmt.Sprintf("panic: %v", r)); err != nil {
				m.logger.WithField("error", err).Error("set failed status")
			}
		}
	}()

	// Run the preflights using the shared core function
	output, stderr, err := preflights.Run(ctx, opts.HostPreflightSpec, opts.Proxy, opts.RuntimeConfig)
	if err != nil {
		errMsg := fmt.Sprintf("Host preflights failed to run: %v", err)
		if stderr != "" {
			errMsg += fmt.Sprintf(" (stderr: %s)", stderr)
		}
		if err := m.setFailedStatus(errMsg); err != nil {
			m.logger.WithField("error", err).Error("set failed status")
		}
		return
	}

	if err := preflights.SaveToDisk(output, opts.RuntimeConfig.PathToEmbeddedClusterSupportFile("host-preflight-results.json")); err != nil {
		m.logger.WithField("error", err).Warn("save preflights output")
	}

	if err := preflights.CopyBundleTo(opts.RuntimeConfig.PathToEmbeddedClusterSupportFile("preflight-bundle.tar.gz")); err != nil {
		m.logger.WithField("error", err).Warn("copy preflight bundle to embedded-cluster support dir")
	}

	if output.HasFail() || output.HasWarn() {
		if m.metricsReporter != nil {
			m.metricsReporter.ReportPreflightsFailed(ctx, output)
		}
	}

	// Set final status based on results
	if output.HasFail() {
		if err := m.setCompletedStatus(types.StateFailed, "Host preflights failed", output); err != nil {
			m.logger.WithField("error", err).Error("set failed status")
		}
	} else {
		if err := m.setCompletedStatus(types.StateSucceeded, "Host preflights passed", output); err != nil {
			m.logger.WithField("error", err).Error("set succeeded status")
		}
	}
}

func (m *hostPreflightManager) setRunningStatus(hpf *troubleshootv1beta2.HostPreflightSpec) error {
	titles, err := m.getTitles(hpf)
	if err != nil {
		return fmt.Errorf("get titles: %w", err)
	}

	if err := m.hostPreflightStore.SetTitles(titles); err != nil {
		return fmt.Errorf("set titles: %w", err)
	}

	if err := m.hostPreflightStore.SetOutput(nil); err != nil {
		return fmt.Errorf("reset output: %w", err)
	}

	if err := m.hostPreflightStore.SetStatus(&types.Status{
		State:       types.StateRunning,
		Description: "Running host preflights",
		LastUpdated: time.Now(),
	}); err != nil {
		return fmt.Errorf("set status: %w", err)
	}

	return nil
}

func (m *hostPreflightManager) setFailedStatus(description string) error {
	m.logger.Error(description)

	return m.hostPreflightStore.SetStatus(&types.Status{
		State:       types.StateFailed,
		Description: description,
		LastUpdated: time.Now(),
	})
}

func (m *hostPreflightManager) setCompletedStatus(state types.State, description string, output *types.HostPreflightOutput) error {
	if err := m.hostPreflightStore.SetOutput(output); err != nil {
		return fmt.Errorf("set output: %w", err)
	}

	return m.hostPreflightStore.SetStatus(&types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
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
