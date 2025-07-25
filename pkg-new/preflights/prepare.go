package preflights

import (
	"context"
	"fmt"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// ErrPreflightsHaveFail is an error returned when we managed to execute the host preflights but
// they contain failures. We use this to differentiate the way we provide user feedback.
var ErrPreflightsHaveFail = metrics.NewErrorNoFail(fmt.Errorf("host preflight failures detected"))

// PrepareOptions contains options for preparing preflights (shared across CLI and API)
type PrepareOptions struct {
	HostPreflightSpec            *v1beta2.HostPreflightSpec
	ReplicatedAppURL             string
	ProxyRegistryURL             string
	AdminConsolePort             int
	LocalArtifactMirrorPort      int
	DataDir                      string
	K0sDataDir                   string
	OpenEBSDataDir               string
	Proxy                        *ecv1beta1.ProxySpec
	PodCIDR                      string
	ServiceCIDR                  string
	GlobalCIDR                   *string
	NodeIP                       string
	IsAirgap                     bool
	TCPConnectionsRequired       []string
	IsJoin                       bool
	IsUI                         bool
	ControllerAirgapStorageSpace string
	WorkerAirgapStorageSpace     string
}

// Prepare prepares the host preflights spec by merging provided spec with cluster preflights
func (p *PreflightsRunner) Prepare(ctx context.Context, opts PrepareOptions) (*v1beta2.HostPreflightSpec, error) {
	hpf := opts.HostPreflightSpec
	if hpf == nil {
		hpf = &v1beta2.HostPreflightSpec{}
	}

	data, err := types.TemplateData{
		ReplicatedAppURL:             opts.ReplicatedAppURL,
		ProxyRegistryURL:             opts.ProxyRegistryURL,
		IsAirgap:                     opts.IsAirgap,
		AdminConsolePort:             opts.AdminConsolePort,
		LocalArtifactMirrorPort:      opts.LocalArtifactMirrorPort,
		DataDir:                      opts.DataDir,
		K0sDataDir:                   opts.K0sDataDir,
		OpenEBSDataDir:               opts.OpenEBSDataDir,
		SystemArchitecture:           helpers.ClusterArch(),
		FromCIDR:                     opts.PodCIDR,
		ToCIDR:                       opts.ServiceCIDR,
		TCPConnectionsRequired:       opts.TCPConnectionsRequired,
		NodeIP:                       opts.NodeIP,
		IsJoin:                       opts.IsJoin,
		IsUI:                         opts.IsUI,
		ControllerAirgapStorageSpace: opts.ControllerAirgapStorageSpace,
		WorkerAirgapStorageSpace:     opts.WorkerAirgapStorageSpace,
	}.WithCIDRData(opts.PodCIDR, opts.ServiceCIDR, opts.GlobalCIDR)

	if err != nil {
		return nil, fmt.Errorf("get host preflights data: %w", err)
	}

	if opts.Proxy != nil {
		data.HTTPProxy = opts.Proxy.HTTPProxy
		data.HTTPSProxy = opts.Proxy.HTTPSProxy
		data.ProvidedNoProxy = opts.Proxy.ProvidedNoProxy
		data.NoProxy = opts.Proxy.NoProxy
	}

	chpfs, err := GetClusterHostPreflights(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("get cluster host preflights: %w", err)
	}

	for _, h := range chpfs {
		hpf.Collectors = append(hpf.Collectors, h.Spec.Collectors...)
		hpf.Analyzers = append(hpf.Analyzers, h.Spec.Analyzers...)
	}

	return hpf, nil
}
