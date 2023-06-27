package controller

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"

	"github.com/replicatedhq/helmbin/pkg/assets"
	"github.com/replicatedhq/helmbin/pkg/config"
	"github.com/replicatedhq/helmbin/pkg/supervisor"
	"github.com/replicatedhq/helmbin/static"
)

// K0sController implements the component interface to run the k0s controller.
type K0sController struct {
	Options config.K0sControllerOptions

	supervisor supervisor.Supervisor
	Output     io.Writer
	uid        int
	gid        int
}

// Init initializes k0s.
func (k *K0sController) Init(_ context.Context) error {
	if err := assets.Stage(static.FS(), k.Options.DataDir, "bin/k0s", 0550); err != nil {
		return fmt.Errorf("failed to stage k0s: %w", err)
	}
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	k.uid, _ = strconv.Atoi(usr.Uid)
	k.gid, _ = strconv.Atoi(usr.Gid)
	if err := k.writeConfigFile(); err != nil {
		return fmt.Errorf("failed to write k0s config file: %w", err)
	}
	args := []string{
		"controller",
		fmt.Sprintf("--data-dir=%s", filepath.Join(k.Options.DataDir, "k0s")),
		fmt.Sprintf("--config=%s", k.Options.ConfigFile()),
	}
	if k.Options.Debug {
		args = append(args, "--debug")
	}
	if k.Options.EnableWorker {
		args = append(args, "--enable-worker")
	}
	if k.Options.NoTaints {
		args = append(args, "--no-taints")
	}
	if k.Options.TokenFile != "" {
		args = append(args, fmt.Sprintf("--token-file=%s", k.Options.TokenFile))
	}
	if k.Options.CmdLogLevels != nil {
		args = append(args, fmt.Sprintf("--logging=%s", createS2SFlag(k.Options.CmdLogLevels)))
	}
	k.supervisor = supervisor.Supervisor{
		Name:          "k0s",
		UID:           k.uid,
		GID:           k.gid,
		BinPath:       assets.BinPath("k0s", k.Options.BinDir()),
		Output:        k.Output,
		RunDir:        k.Options.RunDir(),
		DataDir:       k.Options.DataDir,
		KeepEnvPrefix: true,
		Args:          args,
	}
	return nil
}

// Start starts k0s.
func (k *K0sController) Start(_ context.Context) error {
	return k.supervisor.Supervise()
}

// Stop stops k0s
func (k *K0sController) Stop() error {
	return k.supervisor.Stop()
}

// Ready is the health-check interface.
func (k *K0sController) Ready() error {
	kubeconfig := filepath.Join(k.Options.DataDir, "k0s/pki/admin.conf")
	_ = os.Setenv("KUBECONFIG", kubeconfig)
	config, err := kconfig.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	res, err := cli.Discovery().RESTClient().Get().AbsPath("/readyz").DoRaw(context.Background())
	if err != nil {
		return fmt.Errorf("failed to reach kubernetes readyz endpoint: %w", err)
	}
	if !bytes.Equal(res, []byte("ok")) {
		return fmt.Errorf("kubernetes readyz endpoint returned %s", res)
	}
	return nil
}

// writeConfigFile writes the k0s config file under Options.CfgFile location.
func (k *K0sController) writeConfigFile() error {
	if err := os.MkdirAll(filepath.Dir(k.Options.ConfigFile()), 0755); err != nil {
		return fmt.Errorf("failed to create dir %s: %w", filepath.Dir(k.Options.ConfigFile()), err)
	}
	in, err := mergeK0sConfigFiles(k.Options.CfgFile)
	if err != nil {
		return fmt.Errorf("failed to merge config files: %w", err)
	}
	err = os.WriteFile(k.Options.ConfigFile(), in, 0644)
	if err != nil {
		return fmt.Errorf("failed to write k0s config file %s: %w", k.Options.ConfigFile(), err)
	}
	return nil
}

// mergeK0sConfigFiles merges a user provided config file with the default one.
func mergeK0sConfigFiles(cfgFile string) ([]byte, error) {
	originalB, err := static.FS().ReadFile("config/k0s.yaml")
	if err != nil {
		return nil, fmt.Errorf("read config template: %w", err)
	}
	if cfgFile == "" {
		return originalB, nil
	}
	patchB, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("read user provided config file %s: %w", cfgFile, err)
	}
	patch := make(strategicpatch.JSONMap)
	err = yaml.UnmarshalStrict(patchB, &patch)
	if err != nil {
		return nil, fmt.Errorf("unmarshal user provided config file %s: %w", cfgFile, err)
	}
	original := make(strategicpatch.JSONMap)
	err = yaml.UnmarshalStrict(originalB, &original)
	if err != nil {
		return nil, fmt.Errorf("unmarshal config template: %w", err)
	}
	merged, err := strategicpatch.StrategicMergeMapPatch(original, patch, v1beta1.ClusterConfig{})
	if err != nil {
		return nil, fmt.Errorf("merge config files: %w", err)
	}
	out, err := yaml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("marshal merged config file: %w", err)
	}
	return out, nil
}

func createS2SFlag(vals map[string]string) string {
	records := make([]string, 0, len(vals)>>1)
	for k, v := range vals {
		records = append(records, k+"="+v)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(records); err != nil {
		panic(err)
	}
	w.Flush()
	return strings.TrimSpace(buf.String())
}
