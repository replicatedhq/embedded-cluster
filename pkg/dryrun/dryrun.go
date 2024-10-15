package dryrun

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	metricstypes "github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/urfave/cli/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	dr *types.DryRun
	mu sync.Mutex
)

const (
	dryRunFile = "ec-dryrun.yaml"
)

func init() {
	if !IsDryRun() {
		return
	}
	dr = &types.DryRun{
		Flags:             map[string]interface{}{},
		Commands:          []types.Command{},
		Metrics:           []metricstypes.Event{},
		HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{},
	}
}

func Dump() error {
	mu.Lock()
	defer mu.Unlock()

	output, err := yaml.Marshal(dr)
	if err != nil {
		return fmt.Errorf("marshal dry run info: %w", err)
	}
	if err := os.WriteFile(dryRunFile, output, 0644); err != nil {
		return fmt.Errorf("write dry run info to file: %w", err)
	}
	return nil
}

func RecordFlags(c *cli.Context) {
	mu.Lock()
	defer mu.Unlock()

	for _, flag := range c.Command.Flags {
		for _, name := range flag.Names() {
			dr.Flags[name] = c.String(name)
		}
	}
}

func RecordCommand(cmd string, args []string, env map[string]string) {
	mu.Lock()
	defer mu.Unlock()

	fullCmd := cmd
	if len(args) > 0 {
		fullCmd += " " + strings.Join(args, " ")
	}
	dr.Commands = append(dr.Commands, types.Command{
		Cmd: fullCmd,
		Env: env,
	})
}

func RecordMetric(metric interface{}) {
	mu.Lock()
	defer mu.Unlock()

	dr.Metrics = append(dr.Metrics, metric)
}

func RecordHostPreflightSpec(hpf *troubleshootv1beta2.HostPreflightSpec) {
	mu.Lock()
	defer mu.Unlock()

	dr.HostPreflightSpec = hpf
}

func KubeClient() (client.Client, error) {
	return dr.KubeClient()
}

func IsDryRun() bool {
	return os.Getenv("EC_DRY_RUN") == "true"
}
