package dryrun

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	dr     *types.DryRun
	drFile string
	mu     sync.Mutex
)

type Client struct {
	KubeUtils *KubeUtils
	Helpers   *Helpers
	Metrics   *Sender
	K0sClient *K0sClient
}

func Init(outputFile string, client *Client) {
	dr = &types.DryRun{
		Flags:             map[string]interface{}{},
		Commands:          []types.Command{},
		Metrics:           []types.Metric{},
		HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{},
	}
	drFile = outputFile
	if client == nil {
		client = &Client{}
	}
	if client.KubeUtils == nil {
		client.KubeUtils = &KubeUtils{}
	}
	if client.Helpers == nil {
		client.Helpers = &Helpers{}
	}
	if client.Metrics == nil {
		client.Metrics = &Sender{}
	}
	if client.K0sClient == nil {
		client.K0sClient = &K0sClient{}
	}
	kubeutils.Set(client.KubeUtils)
	helpers.Set(client.Helpers)
	metrics.Set(client.Metrics)
	k0s.Set(client.K0sClient)
}

func Dump() error {
	mu.Lock()
	defer mu.Unlock()

	output, err := yaml.Marshal(dr)
	if err != nil {
		return fmt.Errorf("marshal dry run info: %w", err)
	}
	if err := os.WriteFile(drFile, output, 0644); err != nil {
		return fmt.Errorf("write dry run info to file: %w", err)
	}
	return nil
}

func Load() (*types.DryRun, error) {
	data, err := os.ReadFile(drFile)
	if err != nil {
		return nil, fmt.Errorf("read dry run file: %w", err)
	}

	dr := &types.DryRun{}
	if err := yaml.Unmarshal(data, dr); err != nil {
		return nil, fmt.Errorf("unmarshal dry run file: %w", err)
	}
	return dr, nil
}

func RecordFlags(flagSet *pflag.FlagSet) {
	mu.Lock()
	defer mu.Unlock()

	flagSet.VisitAll(func(flag *pflag.Flag) {
		// Store the flag name and its value
		dr.Flags[flag.Name] = flag.Value.String()
	})
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

func RecordMetric(title string, url string, payload []byte) {
	mu.Lock()
	defer mu.Unlock()

	dr.Metrics = append(dr.Metrics, types.Metric{
		Title:   title,
		URL:     url,
		Payload: string(payload),
	})
}

func RecordHostPreflightSpec(hpf *troubleshootv1beta2.HostPreflightSpec) {
	mu.Lock()
	defer mu.Unlock()

	dr.HostPreflightSpec = hpf
}

func KubeClient() (client.Client, error) {
	return dr.KubeClient()
}

func Enabled() bool {
	return dr != nil
}
