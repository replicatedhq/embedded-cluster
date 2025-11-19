package preflight

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	troubleshootanalyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type RunAppPreflightOptions struct {
	AppPreflightSpec *troubleshootv1beta2.PreflightSpec
	RunOptions       preflights.RunOptions
}

func (m *appPreflightManager) RunAppPreflights(ctx context.Context, opts RunAppPreflightOptions) (*types.PreflightsOutput, error) {
	titles, err := m.getTitles(opts.AppPreflightSpec)
	if err != nil {
		return nil, fmt.Errorf("get titles: %w", err)
	}

	if err := m.appPreflightStore.SetTitles(titles); err != nil {
		return nil, fmt.Errorf("set titles: %w", err)
	}

	if err := m.appPreflightStore.SetOutput(nil); err != nil {
		return nil, fmt.Errorf("reset output: %w", err)
	}

	// Run the app preflights using the shared core function
	output, stderr, err := m.runner.RunAppPreflights(ctx, opts.AppPreflightSpec, opts.RunOptions)
	if err != nil {
		if stderr != "" {
			return nil, fmt.Errorf("app preflights failed to run: %w (stderr: %s)", err, stderr)
		}
		return nil, fmt.Errorf("app preflights failed to run: %w", err)
	}

	// Set final status based on results
	if err := m.appPreflightStore.SetOutput(output); err != nil {
		return nil, fmt.Errorf("set output: %w", err)
	}

	return output, nil
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

func (m *appPreflightManager) ClearAppPreflightResults(ctx context.Context) error {
	return m.appPreflightStore.Clear()
}
