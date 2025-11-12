package preflights

import (
	"context"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

var p PreflightRunnerInterface

func init() {
	Set(New())
}

func Set(_p PreflightRunnerInterface) {
	p = _p
}

// RunOptions contains options for running preflights without requiring RuntimeConfig
type RunOptions struct {
	PreflightBinaryPath string
	ProxySpec           *ecv1beta1.ProxySpec
	ExtraPaths          []string
}

type PreflightRunnerInterface interface {
	RunHostPreflights(ctx context.Context, spec *troubleshootv1beta2.HostPreflightSpec, opts RunOptions) (*apitypes.PreflightsOutput, string, error)
	RunAppPreflights(ctx context.Context, spec *troubleshootv1beta2.PreflightSpec, opts RunOptions) (*apitypes.PreflightsOutput, string, error)
}

// Convenience functions
// TODO: can be removed once all consumers use the interface directly

func RunHostPreflights(ctx context.Context, spec *troubleshootv1beta2.HostPreflightSpec, opts RunOptions) (*apitypes.PreflightsOutput, string, error) {
	return p.RunHostPreflights(ctx, spec, opts)
}

func RunAppPreflights(ctx context.Context, spec *troubleshootv1beta2.PreflightSpec, opts RunOptions) (*apitypes.PreflightsOutput, string, error) {
	return p.RunAppPreflights(ctx, spec, opts)
}
