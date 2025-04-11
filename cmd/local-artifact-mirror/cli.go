package main

import (
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CLI struct {
	Name              string
	V                 *viper.Viper
	KCLIGetter        func() (client.Client, error)
	PullArtifact      PullArtifactFunc
	ServeRequiresRoot bool
}

func NewCLI(name string) *CLI {
	cli := &CLI{
		Name:              name,
		V:                 viper.New(),
		KCLIGetter:        kubeutils.KubeClient,
		PullArtifact:      pullArtifact,
		ServeRequiresRoot: true,
	}
	return cli
}

func (cli *CLI) init() {
	cli.V.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	cli.V.SetEnvPrefix("REPLICATED")
	cli.V.AutomaticEnv()
}

func (cli *CLI) bindFlags(flags *pflag.FlagSet) {
	cli.V.BindPFlags(flags)
}

// setupDataDir configures the data directory and TMPDIR environment variable.
// It handles environment variables for backwards compatibility.
func (cli *CLI) setupDataDir() {
	dataDir := cli.V.GetString("data-dir")
	if dataDir != "" {
		runtimeconfig.SetDataDir(dataDir)
	}

	os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())
}
