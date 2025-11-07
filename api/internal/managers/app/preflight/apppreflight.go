package preflight

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	troubleshootanalyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type RunAppPreflightOptions struct {
	AppPreflightSpec *troubleshootv1beta2.PreflightSpec
	RunOptions       preflights.RunOptions
}

func (m *appPreflightManager) RunAppPreflights(ctx context.Context, opts RunAppPreflightOptions) (finalErr error) {
	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))

			if err := m.setFailedStatus("App preflights failed to run: panic"); err != nil {
				m.logger.WithError(err).Error("set failed status")
			}
		}
	}()

	if err := m.setRunningStatus(opts.AppPreflightSpec); err != nil {
		return fmt.Errorf("set running status: %w", err)
	}

	// TODO: use dependency injection for the preflights runner
	if dryrun.Enabled() {
		if err := m.setCompletedStatus(types.StateSucceeded, "App preflights passed", nil); err != nil {
			return fmt.Errorf("set succeeded status: %w", err)
		}

		dryrun.RecordAppPreflightSpec(opts.AppPreflightSpec)
		return nil
	}

	// Run the app preflights using the shared core function
	output, stderr, err := m.runner.RunAppPreflights(ctx, opts.AppPreflightSpec, opts.RunOptions)
	if err != nil {
		errMsg := fmt.Sprintf("App preflights failed to run: %v", err)
		if stderr != "" {
			errMsg += fmt.Sprintf(" (stderr: %s)", stderr)
		}
		m.logger.Error(errMsg)
		if err := m.setFailedStatus(errMsg); err != nil {
			return fmt.Errorf("set failed status: %w", err)
		}
		return
	}

	// Set final status based on results
	if output.HasFail() {
		if err := m.setCompletedStatus(types.StateFailed, "App preflights failed", output); err != nil {
			return fmt.Errorf("set failed status: %w", err)
		}
	} else {
		if err := m.setCompletedStatus(types.StateSucceeded, "App preflights passed", output); err != nil {
			return fmt.Errorf("set succeeded status: %w", err)
		}
	}

	return nil
}

func (m *appPreflightManager) GetAppPreflightStatus(ctx context.Context) (types.Status, error) {
	return m.appPreflightStore.GetStatus()
}

func (m *appPreflightManager) GetAppPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error) {
	return m.appPreflightStore.GetOutput()
}

func (m *appPreflightManager) GetAppPreflightTitles(ctx context.Context) ([]string, error) {
	return m.appPreflightStore.GetTitles()
}

func (m *appPreflightManager) setRunningStatus(apf *troubleshootv1beta2.PreflightSpec) error {
	titles, err := m.getTitles(apf)
	if err != nil {
		return fmt.Errorf("get titles: %w", err)
	}

	if err := m.appPreflightStore.SetTitles(titles); err != nil {
		return fmt.Errorf("set titles: %w", err)
	}

	if err := m.appPreflightStore.SetOutput(nil); err != nil {
		return fmt.Errorf("reset output: %w", err)
	}

	if err := m.appPreflightStore.SetStatus(types.Status{
		State:       types.StateRunning,
		Description: "Running app preflights",
		LastUpdated: time.Now(),
	}); err != nil {
		return fmt.Errorf("set status: %w", err)
	}

	return nil
}

func (m *appPreflightManager) setFailedStatus(description string) error {
	return m.appPreflightStore.SetStatus(types.Status{
		State:       types.StateFailed,
		Description: description,
		LastUpdated: time.Now(),
	})
}

func (m *appPreflightManager) setCompletedStatus(state types.State, description string, output *types.PreflightsOutput) error {
	if err := m.appPreflightStore.SetOutput(output); err != nil {
		return fmt.Errorf("set output: %w", err)
	}

	return m.appPreflightStore.SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}

func (m *appPreflightManager) getTitles(apf *troubleshootv1beta2.PreflightSpec) ([]string, error) {
	if apf == nil || apf.Analyzers == nil {
		return nil, nil
	}

	titles := []string{}
	for _, a := range apf.Analyzers {
		analyzer := troubleshootanalyze.GetAnalyzer(a)
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
