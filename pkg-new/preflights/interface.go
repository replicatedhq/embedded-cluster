package preflights

import (
	"context"
	"io"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

var p PreflightsRunnerInterface

func init() {
	Set(New())
}

func Set(_p PreflightsRunnerInterface) {
	p = _p
}

type PreflightsRunnerInterface interface {
	Prepare(ctx context.Context, opts PrepareOptions) (*troubleshootv1beta2.HostPreflightSpec, error)
	Run(ctx context.Context, spec *troubleshootv1beta2.HostPreflightSpec, rc runtimeconfig.RuntimeConfig) (*apitypes.HostPreflightsOutput, string, error)
	CopyBundleTo(dst string) error
	SaveToDisk(output *apitypes.HostPreflightsOutput, path string) error
	OutputFromReader(reader io.Reader) (*apitypes.HostPreflightsOutput, error)
	PrintTable(o *apitypes.HostPreflightsOutput)
	PrintTableWithoutInfo(o *apitypes.HostPreflightsOutput)
}

// Convenience functions
// TODO: can be removed once all consumers use the interface directly

func Prepare(ctx context.Context, opts PrepareOptions) (*troubleshootv1beta2.HostPreflightSpec, error) {
	return p.Prepare(ctx, opts)
}

func Run(ctx context.Context, spec *troubleshootv1beta2.HostPreflightSpec, rc runtimeconfig.RuntimeConfig) (*apitypes.HostPreflightsOutput, string, error) {
	return p.Run(ctx, spec, rc)
}

func CopyBundleTo(dst string) error {
	return p.CopyBundleTo(dst)
}

func SaveToDisk(output *apitypes.HostPreflightsOutput, path string) error {
	return p.SaveToDisk(output, path)
}

func OutputFromReader(reader io.Reader) (*apitypes.HostPreflightsOutput, error) {
	return p.OutputFromReader(reader)
}

func PrintTable(o *apitypes.HostPreflightsOutput) {
	p.PrintTable(o)
}

func PrintTableWithoutInfo(o *apitypes.HostPreflightsOutput) {
	p.PrintTableWithoutInfo(o)
}
