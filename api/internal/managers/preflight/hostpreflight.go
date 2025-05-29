package preflight

import (
	"context"
	"fmt"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/paths"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	preflighttypes "github.com/replicatedhq/embedded-cluster/pkg-new/preflights/types"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	troubleshootanalyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type PrepareHostPreflightOptions struct {
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
	HostPreflightSpec *troubleshootv1beta2.HostPreflightSpec
	Proxy             *ecv1beta1.ProxySpec
	DataDirectory     string
}

func (m *hostPreflightManager) PrepareHostPreflights(ctx context.Context, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, *ecv1beta1.ProxySpec, error) {
	hpf, proxy, err := m.prepareHostPreflights(ctx, opts)
	if err != nil {
		return nil, nil, err
	}
	return hpf, proxy, nil
}

func (m *hostPreflightManager) RunHostPreflights(ctx context.Context, opts RunHostPreflightOptions) (*types.RunHostPreflightResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return nil, fmt.Errorf("host preflights are already running")
	}

	titles, err := m.getTitles(opts.HostPreflightSpec)
	if err != nil {
		return nil, fmt.Errorf("get titles: %w", err)
	}

	m.isRunning = true
	m.status = types.HostPreflightStatus{
		State:       types.HostPreflightStateRunning,
		Description: "Running host preflights",
		LastUpdated: time.Now(),
	}
	m.output = nil

	// Run preflights in background
	go m.runHostPreflights(ctx, opts)

	return &types.RunHostPreflightResponse{
		Status: m.status,
		Titles: titles,
	}, nil
}

func (m *hostPreflightManager) GetHostPreflightStatus(ctx context.Context) (*types.HostPreflightStatusResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	response := &types.HostPreflightStatusResponse{
		Status: m.status,
		Output: m.output,
	}

	return response, nil
}

func (m *hostPreflightManager) prepareHostPreflights(ctx context.Context, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, *ecv1beta1.ProxySpec, error) {
	// Use provided installation config
	config := opts.InstallationConfig
	if config == nil {
		return nil, nil, fmt.Errorf("installation config is required")
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
		// TODO (@salah): should we handle K0sDataDirOverride & OpenEBSDataDirOverride when we support running preflights on upgrade from old versions?
		K0sDataDir:             paths.K0sSubDir(opts.InstallationConfig.DataDirectory),
		OpenEBSDataDir:         paths.OpenEBSLocalSubDir(opts.InstallationConfig.DataDirectory),
		Proxy:                  proxy,
		PodCIDR:                config.PodCIDR,
		ServiceCIDR:            config.ServiceCIDR,
		GlobalCIDR:             globalCIDR,
		NodeIP:                 nodeIP,
		IsAirgap:               opts.IsAirgap,
		TCPConnectionsRequired: opts.TCPConnectionsRequired,
		IsJoin:                 opts.IsJoin,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("prepare host preflights: %w", err)
	}

	return hpf, proxy, nil
}

func (m *hostPreflightManager) runHostPreflights(ctx context.Context, opts RunHostPreflightOptions) {
	defer func() {
		m.mu.Lock()
		m.isRunning = false
		m.mu.Unlock()
	}()

	defer func() {
		if r := recover(); r != nil {
			m.setFailedStatus(fmt.Sprintf("Panic: %v", r))
		}
	}()

	// Run the preflights using the shared core function
	output, stderr, err := preflights.Run(ctx, opts.HostPreflightSpec, opts.Proxy)
	if err != nil {
		errMsg := fmt.Sprintf("Host preflights failed to run: %v", err)
		if stderr != "" {
			errMsg += fmt.Sprintf(" (stderr: %s)", stderr)
		}
		m.setFailedStatus(errMsg)
		return
	}

	if err := output.SaveToDisk(paths.SupportFilePath(opts.DataDirectory, "host-preflight-results.json")); err != nil {
		m.logger.WithField("error", err).Warn("save preflights output")
	}

	if err := preflights.CopyBundleTo(paths.SupportFilePath(opts.DataDirectory, "preflight-bundle.tar.gz")); err != nil {
		m.logger.WithField("error", err).Warn("copy preflight bundle to embedded-cluster support dir")
	}

	// TODO (@salah): report bypassing preflights on a separate api endpoint if the user chooses to bypass and continue
	if output.HasFail() || output.HasWarn() {
		if m.metricsReporter != nil {
			m.metricsReporter.ReportPreflightsFailed(ctx, *output)
		}
	}

	// Convert output to API format
	apiOutput := m.convertOutputToAPI(output)

	// Set final status based on results
	if output.HasFail() {
		m.setCompletedStatus(types.HostPreflightStateFailed, "Host preflights failed", apiOutput)
	} else {
		m.setCompletedStatus(types.HostPreflightStateSucceeded, "Host preflights completed successfully", apiOutput)
	}
}

func (m *hostPreflightManager) setFailedStatus(description string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status = types.HostPreflightStatus{
		State:       types.HostPreflightStateFailed,
		Description: description,
		LastUpdated: time.Now(),
	}
	m.logger.Error(description)
}

func (m *hostPreflightManager) setCompletedStatus(state types.HostPreflightState, description string, output *types.HostPreflightOutput) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status = types.HostPreflightStatus{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	}
	m.output = output
}

func (m *hostPreflightManager) convertOutputToAPI(output *preflighttypes.Output) *types.HostPreflightOutput {
	if output == nil {
		return nil
	}

	apiOutput := &types.HostPreflightOutput{
		Pass: make([]types.HostPreflightRecord, len(output.Pass)),
		Warn: make([]types.HostPreflightRecord, len(output.Warn)),
		Fail: make([]types.HostPreflightRecord, len(output.Fail)),
	}

	for i, record := range output.Pass {
		apiOutput.Pass[i] = types.HostPreflightRecord{
			Title:   record.Title,
			Message: record.Message,
		}
	}

	for i, record := range output.Warn {
		apiOutput.Warn[i] = types.HostPreflightRecord{
			Title:   record.Title,
			Message: record.Message,
		}
	}

	for i, record := range output.Fail {
		apiOutput.Fail[i] = types.HostPreflightRecord{
			Title:   record.Title,
			Message: record.Message,
		}
	}

	return apiOutput
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
