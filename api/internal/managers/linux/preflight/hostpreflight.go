package preflight

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootanalyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type PrepareHostPreflightOptions struct {
	ReplicatedAppURL             string
	ProxyRegistryURL             string
	HostPreflightSpec            *troubleshootv1beta2.HostPreflightSpec
	EmbeddedClusterConfig        *ecv1beta1.Config
	TCPConnectionsRequired       []string
	IsAirgap                     bool
	IsJoin                       bool
	IsUI                         bool
	ControllerAirgapStorageSpace string
	WorkerAirgapStorageSpace     string
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

	// Use the shared Prepare function to prepare host preflights
	prepareOpts := preflights.PrepareOptions{
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
		ControllerAirgapStorageSpace: opts.ControllerAirgapStorageSpace,
		WorkerAirgapStorageSpace:     opts.WorkerAirgapStorageSpace,
	}
	if cidr := rc.GlobalCIDR(); cidr != "" {
		prepareOpts.GlobalCIDR = &cidr
	}

	// Use the shared Prepare function to prepare host preflights
	hpf, err := m.runner.Prepare(ctx, prepareOpts)
	if err != nil {
		return nil, fmt.Errorf("prepare host preflights: %w", err)
	}

	return hpf, nil
}

func (m *hostPreflightManager) RunHostPreflights(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts RunHostPreflightOptions) (finalErr error) {
	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))

			if err := m.setFailedStatus("Host preflights failed to run: panic"); err != nil {
				m.logger.WithError(err).Error("set failed status")
			}
		}
	}()

	if err := m.setRunningStatus(opts.HostPreflightSpec); err != nil {
		return fmt.Errorf("set running status: %w", err)
	}

	// Run the preflights using the shared core function
	output, stderr, err := m.runner.Run(ctx, opts.HostPreflightSpec, rc)
	if err != nil {
		errMsg := fmt.Sprintf("Host preflights failed to run: %v", err)
		if stderr != "" {
			errMsg += fmt.Sprintf(" (stderr: %s)", stderr)
		}
		m.logger.Error(errMsg)
		if err := m.setFailedStatus(errMsg); err != nil {
			return fmt.Errorf("set failed status: %w", err)
		}
		return
	}

	if err := m.runner.SaveToDisk(output, rc.PathToEmbeddedClusterSupportFile("host-preflight-results.json")); err != nil {
		m.logger.WithError(err).Warn("save preflights output")
	}

	if err := m.runner.CopyBundleTo(rc.PathToEmbeddedClusterSupportFile("preflight-bundle.tar.gz")); err != nil {
		m.logger.WithError(err).Warn("copy preflight bundle to embedded-cluster support dir")
	}

	// Set final status based on results
	// TODO @jgantunes: we're currently not handling warnings in the output.
	if output.HasFail() {
		if err := m.setCompletedStatus(types.StateFailed, "Host preflights failed", output); err != nil {
			return fmt.Errorf("set failed status: %w", err)
		}
	} else {
		if err := m.setCompletedStatus(types.StateSucceeded, "Host preflights passed", output); err != nil {
			return fmt.Errorf("set succeeded status: %w", err)
		}
	}

	return nil
}

func (m *hostPreflightManager) GetHostPreflightStatus(ctx context.Context) (types.Status, error) {
	return m.hostPreflightStore.GetStatus()
}

func (m *hostPreflightManager) GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightsOutput, error) {
	return m.hostPreflightStore.GetOutput()
}

func (m *hostPreflightManager) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	return m.hostPreflightStore.GetTitles()
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

	if err := m.hostPreflightStore.SetStatus(types.Status{
		State:       types.StateRunning,
		Description: "Running host preflights",
		LastUpdated: time.Now(),
	}); err != nil {
		return fmt.Errorf("set status: %w", err)
	}

	return nil
}

func (m *hostPreflightManager) setFailedStatus(description string) error {
	return m.hostPreflightStore.SetStatus(types.Status{
		State:       types.StateFailed,
		Description: description,
		LastUpdated: time.Now(),
	})
}

func (m *hostPreflightManager) setCompletedStatus(state types.State, description string, output *types.HostPreflightsOutput) error {
	if err := m.hostPreflightStore.SetOutput(output); err != nil {
		return fmt.Errorf("set output: %w", err)
	}

	return m.hostPreflightStore.SetStatus(types.Status{
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
