package dryrun

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	dryRunInfo                     *types.DryRun
	originalStdout, originalStderr *os.File
	mu                             sync.Mutex
)

func init() {
	if !IsDryRun() {
		return
	}
	dryRunInfo = &types.DryRun{
		Flags:    map[string]interface{}{},
		Commands: []types.Command{},
		Metrics:  []interface{}{},
		Data:     map[string]interface{}{},
	}
	if err := disableStdoutAndStderr(); err != nil {
		panic(fmt.Errorf("unable to disable stdout and stderr: %w", err))
	}
}

func disableStdoutAndStderr() error {
	originalStdout = os.Stdout
	originalStderr = os.Stderr

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open dev null: %w", err)
	}
	defer devNull.Close()

	os.Stdout = devNull
	os.Stderr = devNull

	return nil
}

func restoreStdoutAndStderr() {
	os.Stdout = originalStdout
	os.Stderr = originalStderr
}

func Dump() error {
	mu.Lock()
	defer mu.Unlock()

	marshalled, err := json.MarshalIndent(dryRunInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dry run info: %w", err)
	}

	restoreStdoutAndStderr()
	fmt.Println(string(marshalled))

	return nil
}

func RecordFlags(c *cli.Context) {
	mu.Lock()
	defer mu.Unlock()

	for _, flag := range c.Command.Flags {
		for _, name := range flag.Names() {
			dryRunInfo.Flags[name] = c.String(name)
		}
	}
}

func RecordCommand(cmd string, args []string, env map[string]string) {
	mu.Lock()
	defer mu.Unlock()

	dryRunInfo.Commands = append(dryRunInfo.Commands, types.Command{
		Cmd:  cmd,
		Args: args,
		Env:  env,
	})
}

func RecordMetric(metric interface{}) {
	mu.Lock()
	defer mu.Unlock()

	dryRunInfo.Metrics = append(dryRunInfo.Metrics, metric)
}

func RecordData(key string, value interface{}) {
	mu.Lock()
	defer mu.Unlock()

	dryRunInfo.Data[key] = value
}

func KubeClient() (client.Client, error) {
	if dryRunInfo.KCLI == nil {
		scheme := runtime.NewScheme()
		if err := corev1.AddToScheme(scheme); err != nil {
			return nil, fmt.Errorf("add core v1 scheme: %w", err)
		}
		if err := ecv1beta1.SchemeBuilder.AddToScheme(scheme); err != nil {
			return nil, fmt.Errorf("add ec v1beta1 scheme: %w", err)
		}
		dryRunInfo.KCLI = fake.NewClientBuilder().
			WithScheme(scheme).
			Build()
	}
	return dryRunInfo.KCLI, nil
}

func IsDryRun() bool {
	return os.Getenv("EC_DRY_RUN") == "true"
}
