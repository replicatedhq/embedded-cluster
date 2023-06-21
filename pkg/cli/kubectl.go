package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/component-base/logs"
	kubectl "k8s.io/kubectl/pkg/cmd"

	"github.com/replicatedhq/helmbin/pkg/config"
)

// NewCmdKubectl returns a cobra command for running kubectl
func NewCmdKubectl(_ *CLI) *cobra.Command {
	var cliOpts config.CLIOptions
	// Create a new kubectl command without a plugin handler.
	kubectlCmd := kubectl.NewKubectlCommand(kubectl.KubectlOptions{
		IOStreams: genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	})
	// Add some additional kubectl flags:
	persistentFlags := kubectlCmd.PersistentFlags()
	logs.AddFlags(persistentFlags) // This is done by k8s.io/component-base/cli
	cliFlags := config.GetCLIFlags(&cliOpts)
	kubectlCmd.Flags().AddFlagSet(cliFlags)
	originalPreRunE := kubectlCmd.PersistentPreRunE
	kubectlCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := callParentPersistentPreRun(kubectlCmd, args); err != nil {
			return err
		}
		if err := fallbackToK0sKubeconfig(cmd, cliOpts.DataDir); err != nil {
			return err
		}
		return originalPreRunE(cmd, args)
	}
	return kubectlCmd
}

func callParentPersistentPreRun(c *cobra.Command, args []string) error {
	for p := c.Parent(); p != nil; p = p.Parent() {
		preRunE := p.PersistentPreRunE
		preRun := p.PersistentPreRun
		p.PersistentPreRunE = nil
		p.PersistentPreRun = nil
		defer func() {
			p.PersistentPreRunE = preRunE
			p.PersistentPreRun = preRun
		}()
		if preRunE != nil {
			return preRunE(c, args)
		}
		if preRun != nil {
			preRun(c, args)
			return nil
		}
	}
	return nil
}

func fallbackToK0sKubeconfig(cmd *cobra.Command, dataDir string) error {
	kubeconfigFlag := cmd.Flags().Lookup("kubeconfig")
	if kubeconfigFlag == nil {
		return fmt.Errorf("kubeconfig flag not found")
	}
	if kubeconfigFlag.Changed {
		_ = os.Unsetenv("KUBECONFIG")
		return nil
	}
	if _, ok := os.LookupEnv("KUBECONFIG"); ok {
		return nil
	}
	certDir := filepath.Join(dataDir, "k0s/pki")
	adminKubeConfigPath := filepath.Join(certDir, "admin.conf")
	kubeconfig := adminKubeConfigPath
	// verify that k0s's kubeconfig is readable before pushing it to the env
	if _, err := os.Stat(kubeconfig); err != nil {
		return fmt.Errorf("cannot stat k0s kubeconfig, is the server running?: %w", err)
	}
	if err := kubeconfigFlag.Value.Set(kubeconfig); err != nil {
		return fmt.Errorf("failed to set kubeconfig flag: %w", err)
	}
	return nil
}
