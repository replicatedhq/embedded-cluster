package preflights

import (
	"context"
	"fmt"
	"runtime"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights/types"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
)

// ErrPreflightsHaveFail is an error returned when we managed to execute the host preflights but
// they contain failures. We use this to differentiate the way we provide user feedback.
var ErrPreflightsHaveFail = metrics.NewErrorNoFail(fmt.Errorf("host preflight failures detected"))

type PrepareAndRunOptions struct {
	ReplicatedAPIURL       string
	ProxyRegistryURL       string
	Proxy                  *ecv1beta1.ProxySpec
	PodCIDR                string
	ServiceCIDR            string
	GlobalCIDR             *string
	NodeIP                 string
	PrivateCAs             []string
	IsAirgap               bool
	SkipHostPreflights     bool
	IgnoreHostPreflights   bool
	AssumeYes              bool
	TCPConnectionsRequired []string
	MetricsReporter        MetricsReporter
	IsJoin                 bool
}

type MetricsReporter interface {
	ReportPreflightsFailed(ctx context.Context, output types.Output, bypassed bool)
}

func PrepareAndRun(ctx context.Context, opts PrepareAndRunOptions) error {
	hpf := release.GetHostPreflights()
	if hpf == nil {
		hpf = &v1beta2.HostPreflightSpec{}
	}

	privateCA := ""
	if len(opts.PrivateCAs) > 0 {
		privateCA = opts.PrivateCAs[0]
	}

	data, err := types.TemplateData{
		ReplicatedAPIURL:        opts.ReplicatedAPIURL,
		ProxyRegistryURL:        opts.ProxyRegistryURL,
		IsAirgap:                opts.IsAirgap,
		AdminConsolePort:        runtimeconfig.AdminConsolePort(),
		LocalArtifactMirrorPort: runtimeconfig.LocalArtifactMirrorPort(),
		DataDir:                 runtimeconfig.EmbeddedClusterHomeDirectory(),
		K0sDataDir:              runtimeconfig.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:          runtimeconfig.EmbeddedClusterOpenEBSLocalSubDir(),
		PrivateCA:               privateCA,
		SystemArchitecture:      runtime.GOARCH,
		FromCIDR:                opts.PodCIDR,
		ToCIDR:                  opts.ServiceCIDR,
		TCPConnectionsRequired:  opts.TCPConnectionsRequired,
		NodeIP:                  opts.NodeIP,
		IsJoin:                  opts.IsJoin,
	}.WithCIDRData(opts.PodCIDR, opts.ServiceCIDR, opts.GlobalCIDR)

	if err != nil {
		return fmt.Errorf("get host preflights data: %w", err)
	}

	if opts.Proxy != nil {
		data.HTTPProxy = opts.Proxy.HTTPProxy
		data.HTTPSProxy = opts.Proxy.HTTPSProxy
		data.ProvidedNoProxy = opts.Proxy.ProvidedNoProxy
		data.NoProxy = opts.Proxy.NoProxy
	}

	chpfs, err := GetClusterHostPreflights(ctx, data)
	if err != nil {
		return fmt.Errorf("get cluster host preflights: %w", err)
	}

	for _, h := range chpfs {
		hpf.Collectors = append(hpf.Collectors, h.Spec.Collectors...)
		hpf.Analyzers = append(hpf.Analyzers, h.Spec.Analyzers...)
	}

	if dryrun.Enabled() {
		dryrun.RecordHostPreflightSpec(hpf)
		return nil
	}

	return runHostPreflights(ctx, hpf, opts)
}

func runHostPreflights(ctx context.Context, hpf *v1beta2.HostPreflightSpec, opts PrepareAndRunOptions) error {
	if len(hpf.Collectors) == 0 && len(hpf.Analyzers) == 0 {
		return nil
	}

	pb := spinner.Start()

	if opts.SkipHostPreflights {
		pb.Infof("Host preflights skipped")
		pb.Close()
		return nil
	}

	pb.Infof("Running host preflights")

	output, stderr, err := Run(ctx, hpf, opts.Proxy)
	if err != nil {
		pb.CloseWithError()
		return fmt.Errorf("host preflights failed to run: %w", err)
	}
	if stderr != "" {
		logrus.Debugf("preflight stderr: %s", stderr)
	}

	err = output.SaveToDisk(runtimeconfig.PathToEmbeddedClusterSupportFile("host-preflight-results.json"))
	if err != nil {
		logrus.Warnf("save preflights output: %v", err)
	}

	err = CopyBundleToECSupportDir()
	if err != nil {
		logrus.Warnf("copy preflight bundle to embedded-cluster support dir: %v", err)
	}

	// Failures found
	if output.HasFail() {
		s := "preflights"
		if len(output.Fail) == 1 {
			s = "preflight"
		}

		if output.HasWarn() {
			pb.Errorf("%d host %s failed and %d warned", len(output.Fail), s, len(output.Warn))
		} else {
			pb.Errorf("%d host %s failed", len(output.Fail), s)
		}

		pb.CloseWithError()
		output.PrintTableWithoutInfo()

		if opts.IgnoreHostPreflights {
			if opts.AssumeYes {
				if opts.MetricsReporter != nil {
					opts.MetricsReporter.ReportPreflightsFailed(ctx, *output, true)
				}
				return nil
			}
			if prompts.New().Confirm("Are you sure you want to ignore these failures and continue installing?", false) {
				if opts.MetricsReporter != nil {
					opts.MetricsReporter.ReportPreflightsFailed(ctx, *output, true)
				}
				return nil // user continued after host preflights failed
			}
		}

		if len(output.Fail)+len(output.Warn) > 1 {
			logrus.Info("Please address these issues and try again.")
		} else {
			logrus.Info("Please address this issue and try again.")
		}

		if opts.MetricsReporter != nil {
			opts.MetricsReporter.ReportPreflightsFailed(ctx, *output, true)
		}
		return ErrPreflightsHaveFail
	}

	// Warnings found
	if output.HasWarn() {
		s := "preflights"
		if len(output.Warn) == 1 {
			s = "preflight"
		}

		pb.Warnf("%d host %s warned", len(output.Warn), s)
		if opts.AssumeYes {
			// We have warnings but we are not in interactive mode
			// so we just print the warnings and continue
			pb.Close()
			output.PrintTableWithoutInfo()
			if opts.MetricsReporter != nil {
				opts.MetricsReporter.ReportPreflightsFailed(ctx, *output, true)
			}
			return nil
		}

		pb.Close()
		output.PrintTableWithoutInfo()

		if !prompts.New().Confirm("Do you want to continue?", false) {
			if opts.MetricsReporter != nil {
				opts.MetricsReporter.ReportPreflightsFailed(ctx, *output, true)
			}
			return ErrPreflightsHaveFail
		}

		if opts.MetricsReporter != nil {
			opts.MetricsReporter.ReportPreflightsFailed(ctx, *output, true)
		}
		return nil
	}

	// No failures or warnings
	pb.Infof("Host preflights succeeded!")
	pb.Close()

	return nil
}
