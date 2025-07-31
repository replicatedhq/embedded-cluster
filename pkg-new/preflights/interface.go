package preflights

import (
	"context"
	"io"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

var p PreflightsRunnerInterface

func init() {
	Set(New())
}

func Set(_p PreflightsRunnerInterface) {
	p = _p
}

// RunOptions contains options for running preflights without requiring RuntimeConfig
type RunOptions struct {
	PreflightBinaryPath string
	ProxySpec           *ecv1beta1.ProxySpec
	ExtraPaths          []string
}

type PreflightsRunnerInterface interface {
	PrepareHostPreflights(ctx context.Context, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, error)
	RunHostPreflights(ctx context.Context, spec *troubleshootv1beta2.HostPreflightSpec, opts RunOptions) (*apitypes.PreflightsOutput, string, error)
	RunAppPreflights(ctx context.Context, spec *troubleshootv1beta2.PreflightSpec, opts RunOptions) (*apitypes.PreflightsOutput, string, error)
	CopyBundleTo(dst string) error
	SaveToDisk(output *apitypes.PreflightsOutput, path string) error
	OutputFromReader(reader io.Reader) (*apitypes.PreflightsOutput, error)
	PrintTable(o *apitypes.PreflightsOutput)
	PrintTableWithoutInfo(o *apitypes.PreflightsOutput)
}

// Convenience functions
// TODO: can be removed once all consumers use the interface directly

func PrepareHostPreflights(ctx context.Context, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, error) {
	return p.PrepareHostPreflights(ctx, opts)
}

func RunHostPreflights(ctx context.Context, spec *troubleshootv1beta2.HostPreflightSpec, opts RunOptions) (*apitypes.PreflightsOutput, string, error) {
	return p.RunHostPreflights(ctx, spec, opts)
}

func RunAppPreflights(ctx context.Context, spec *troubleshootv1beta2.PreflightSpec, opts RunOptions) (*apitypes.PreflightsOutput, string, error) {
	return p.RunAppPreflights(ctx, spec, opts)
}

func CopyBundleTo(dst string) error {
	return p.CopyBundleTo(dst)
}

func SaveToDisk(output *apitypes.PreflightsOutput, path string) error {
	return p.SaveToDisk(output, path)
}

func OutputFromReader(reader io.Reader) (*apitypes.PreflightsOutput, error) {
	return p.OutputFromReader(reader)
}

func PrintTable(o *apitypes.PreflightsOutput) {
	p.PrintTable(o)
}

func PrintTableWithoutInfo(o *apitypes.PreflightsOutput) {
	p.PrintTableWithoutInfo(o)
}
