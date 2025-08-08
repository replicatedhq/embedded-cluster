package cli

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
)

func runHostPreflights(
	ctx context.Context,
	hpf *troubleshootv1beta2.HostPreflightSpec,
	rc runtimeconfig.RuntimeConfig,
	skipHostPreflights bool,
	ignoreHostPreflights bool,
	assumeYes bool,
	metricsReporter metrics.ReporterInterface,
) error {
	if dryrun.Enabled() {
		dryrun.RecordHostPreflightSpec(hpf)
		return nil
	}

	if len(hpf.Collectors) == 0 && len(hpf.Analyzers) == 0 {
		return nil
	}

	spinner := spinner.Start()

	if skipHostPreflights {
		spinner.Closef("Host preflights skipped")
		return nil
	}

	spinner.Infof("Running host preflights")

	opts := preflights.RunOptions{
		PreflightBinaryPath: rc.PathToEmbeddedClusterBinary("kubectl-preflight"),
		ProxySpec:           rc.ProxySpec(),
		ExtraPaths:          []string{rc.EmbeddedClusterBinsSubDir()},
	}
	output, stderr, err := preflights.RunHostPreflights(ctx, hpf, opts)
	if stderr != "" {
		logrus.Debugf("preflight stderr: %s", stderr)
	}
	if err != nil {
		spinner.ErrorClosef("Failed to run host preflights")
		return fmt.Errorf("host preflights failed to run: %w", err)
	}

	err = preflights.SaveToDisk(output, rc.PathToEmbeddedClusterSupportFile("host-preflight-results.json"))
	if err != nil {
		logrus.Warnf("save preflights output: %v", err)
	}

	err = preflights.CopyBundleTo(rc.PathToEmbeddedClusterSupportFile("preflight-bundle.tar.gz"))
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
			spinner.ErrorClosef("%d host %s failed and %d warned", len(output.Fail), s, len(output.Warn))
		} else {
			spinner.ErrorClosef("%d host %s failed", len(output.Fail), s)
		}

		preflights.PrintTableWithoutInfo(output)

		if ignoreHostPreflights {
			if assumeYes {
				if metricsReporter != nil {
					metricsReporter.ReportHostPreflightsBypassed(ctx, output)
				}
				return nil
			}
			confirmed, err := prompts.New().Confirm("Are you sure you want to ignore these failures and continue installing?", false)
			if err != nil {
				return fmt.Errorf("failed to get confirmation: %w", err)
			}
			if confirmed {
				if metricsReporter != nil {
					metricsReporter.ReportHostPreflightsBypassed(ctx, output)
				}
				return nil // user continued after host preflights failed
			}
		}

		if len(output.Fail)+len(output.Warn) > 1 {
			logrus.Info("\n\033[1mPlease address these issues and try again.\033[0m\n")
		} else {
			logrus.Info("\n\033[1mPlease address this issue and try again.\033[0m\n")
		}

		if metricsReporter != nil {
			metricsReporter.ReportHostPreflightsFailed(ctx, output)
		}
		return ErrPreflightsHaveFail
	}

	// Warnings found
	if output.HasWarn() {
		s := "preflights"
		if len(output.Warn) == 1 {
			s = "preflight"
		}

		spinner.Warnf("%d host %s warned", len(output.Warn), s)
		spinner.Close()
		if assumeYes {
			// We have warnings but we are not in interactive mode
			// so we just print the warnings and continue
			preflights.PrintTableWithoutInfo(output)
			if metricsReporter != nil {
				metricsReporter.ReportHostPreflightsBypassed(ctx, output)
			}
			return nil
		}

		preflights.PrintTableWithoutInfo(output)

		confirmed, err := prompts.New().Confirm("Do you want to continue?", false)
		if err != nil {
			return fmt.Errorf("failed to get confirmation: %w", err)
		}
		if !confirmed {
			if metricsReporter != nil {
				metricsReporter.ReportHostPreflightsFailed(ctx, output)
			}
			return ErrPreflightsHaveFail
		}

		if metricsReporter != nil {
			metricsReporter.ReportHostPreflightsBypassed(ctx, output)
		}
		return nil
	}

	// No failures or warnings
	spinner.Infof("Host preflights passed")
	spinner.Close()

	return nil
}
