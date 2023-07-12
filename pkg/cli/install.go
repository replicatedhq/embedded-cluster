package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/k0sproject/k0s/pkg/config"
	k0sinstall "github.com/k0sproject/k0s/pkg/install"
	"github.com/replicatedhq/helmbin/pkg/constants"
	"github.com/replicatedhq/helmbin/pkg/install"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type command config.CLIOptions

type installFlags struct {
	force   bool
	envVars []string
}

// NewCmdInstall returns a cobra command for installing a controller+worker as a systemd service
func NewCmdInstall(cli *CLI) *cobra.Command {
	cmdinstall := newInstallCmd(cli)
	cli.cmdReplaceK0s(cmdinstall)
	for _, cmd := range cmdinstall.Commands() {
		cli.cmdReplaceK0s(cmd)
		if cmd.Use == "controller" {
			cmdinstall.PreRunE = cmd.PreRunE
			cmdinstall.RunE = cmd.RunE
		}
	}
	return cmdinstall
}

func newInstallCmd(cli *CLI) *cobra.Command {
	var installFlags installFlags

	cmd := installCmd(cli, &installFlags)

	cmd.AddCommand(installControllerCmd(cli, &installFlags))
	cmd.AddCommand(installWorkerCmd(cli, &installFlags))
	cmd.PersistentFlags().BoolVar(&installFlags.force, "force", false, "force init script creation")
	cmd.PersistentFlags().StringArrayVarP(&installFlags.envVars, "env", "e", nil, "set environment variable")
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	cmd.Flags().AddFlagSet(config.GetControllerFlags())
	cmd.Flags().AddFlagSet(config.GetWorkerFlags())

	return cmd
}

func installCmd(cli *CLI, installFlags *installFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install k0s on a brand-new system. Must be run as root (or with sudo)",
		Example: `With the install command you can setup a single node cluster by running:

	k0s install
	`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := command(config.GetCmdOpts())
			if err := c.convertFileParamsToAbsolute(); err != nil {
				cmd.SilenceUsage = true
				return err
			}
			flagsAndVals := []string{"controller"}

			// hardcode flags for controller+worker
			err := cmd.Flags().Lookup("enable-worker").Value.Set("true")
			if err != nil {
				panic(err)
			}
			err = cmd.Flags().Lookup("no-taints").Value.Set("true")
			if err != nil {
				panic(err)
			}

			flagsAndVals = append(flagsAndVals, cmdFlagsToArgs(cmd)...)
			if err := c.setup(cli.Name, constants.RoleController, flagsAndVals, installFlags); err != nil {
				cmd.SilenceUsage = true
				return err
			}
			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			c := command(config.GetCmdOpts())
			return config.PreRunValidateConfig(c.K0sVars)
		},
	}
	// append flags
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	cmd.Flags().AddFlagSet(config.GetControllerFlags())
	cmd.Flags().AddFlagSet(config.GetWorkerFlags())

	// hardcode flags for controller+worker
	cmd.Flags().Lookup("enable-worker").Hidden = true
	cmd.Flags().Lookup("no-taints").Hidden = true
	cmd.Flags().Lookup("single").Hidden = true

	return cmd
}

func installControllerCmd(cli *CLI, installFlags *installFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Install k0s controller on a brand-new system. Must be run as root (or with sudo)",
		Example: `With the controller subcommand you can setup a controller node for a multi-node cluster by running:

	k0s install controller
	`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := command(config.GetCmdOpts())
			if err := c.convertFileParamsToAbsolute(); err != nil {
				cmd.SilenceUsage = true
				return err
			}
			flagsAndVals := []string{"controller"}
			flagsAndVals = append(flagsAndVals, cmdFlagsToArgs(cmd)...)
			if err := c.setup(cli.Name, constants.RoleController, flagsAndVals, installFlags); err != nil {
				cmd.SilenceUsage = true
				return err
			}
			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			c := command(config.GetCmdOpts())
			return config.PreRunValidateConfig(c.K0sVars)
		},
	}
	// append flags
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	cmd.Flags().AddFlagSet(config.GetControllerFlags())
	cmd.Flags().AddFlagSet(config.GetWorkerFlags())
	return cmd
}

func installWorkerCmd(cli *CLI, installFlags *installFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Install k0s worker on a brand-new system. Must be run as root (or with sudo)",
		Example: `With the worker subcommand you can setup a worker node for a multi-node cluster by running:

		k0s install worker`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := command(config.GetCmdOpts())
			if err := c.convertFileParamsToAbsolute(); err != nil {
				cmd.SilenceUsage = true
				return err
			}

			flagsAndVals := []string{"worker"}
			flagsAndVals = append(flagsAndVals, cmdFlagsToArgs(cmd)...)
			if err := c.setup(cli.Name, constants.RoleWorker, flagsAndVals, installFlags); err != nil {
				cmd.SilenceUsage = true
				return err
			}

			return nil
		},
	}
	// append flags
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	cmd.PersistentFlags().AddFlagSet(config.GetWorkerFlags())

	return cmd
}

// The setup functions:
//   - Ensures that the proper users are created.
//   - Sets up startup and logging for k0s.
func (c *command) setup(svcName string, role string, args []string, installFlags *installFlags) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root")
	}

	if role == constants.RoleController {
		if err := k0sinstall.CreateControllerUsers(c.NodeConfig, c.K0sVars); err != nil {
			return fmt.Errorf("failed to create controller users: %v", err)
		}
	}
	err := install.EnsureService(svcName, args, installFlags.envVars, installFlags.force)
	if err != nil {
		return fmt.Errorf("failed to install k0s service: %v", err)
	}
	return nil
}

// This command converts the file paths in the command struct to absolute paths.
// For flags passed to service init file, see the [cmdFlagsToArgs] func.
func (c *command) convertFileParamsToAbsolute() (err error) {
	// don't convert if cfgFile is empty

	if c.CfgFile != "" {
		c.CfgFile, err = filepath.Abs(c.CfgFile)
		if err != nil {
			return err
		}
	}
	if c.K0sVars.DataDir != "" {
		c.K0sVars.DataDir, err = filepath.Abs(c.K0sVars.DataDir)
		if err != nil {
			return err
		}
	}
	if c.TokenFile != "" {
		c.TokenFile, err = filepath.Abs(c.TokenFile)
		if err != nil {
			return err
		}
		if !fileExists(c.TokenFile) {
			return fmt.Errorf("%s does not exist", c.TokenFile)
		}
	}
	return nil
}

func cmdFlagsToArgs(cmd *cobra.Command) []string {
	var flagsAndVals []string
	// Use visitor to collect all flags and vals into slice
	cmd.Flags().Visit(func(f *pflag.Flag) {
		val := f.Value.String()
		switch f.Value.Type() {
		case "stringSlice", "stringToString":
			flagsAndVals = append(flagsAndVals, fmt.Sprintf(`--%s=%s`, f.Name, strings.Trim(val, "[]")))
		default:
			if f.Name == "env" || f.Name == "force" {
				return
			}
			if f.Name == "data-dir" || f.Name == "token-file" || f.Name == "config" {
				val, _ = filepath.Abs(val)
			}
			flagsAndVals = append(flagsAndVals, fmt.Sprintf("--%s=%s", f.Name, val))
		}
	})
	return flagsAndVals
}

// fileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func fileExists(fileName string) bool {
	info, err := os.Stat(fileName)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
