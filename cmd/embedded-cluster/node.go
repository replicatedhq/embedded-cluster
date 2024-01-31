package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

var nodeCommands = &cli.Command{
	Name:  "node",
	Usage: "Manage cluster nodes",
	Subcommands: []*cli.Command{
		nodeStopCommand,
		nodeStartCommand,
		nodeListCommand,
		joinCommand,
		resetCommand,
	},
}

var nodeStopCommand = &cli.Command{
	Name:  "stop",
	Usage: "Stops a node",
	Action: func(c *cli.Context) error {
		node := c.Args().First()
		if node == "" {
			return fmt.Errorf("expected node name")
		}
		kcfg := defaults.PathToConfig("kubeconfig")
		os.Setenv("KUBECONFIG", kcfg)
		bin := defaults.PathToEmbeddedClusterBinary("kubectl")
		cmd := exec.Command(bin, "drain", "--ignore-daemonsets", node)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	},
}

var nodeStartCommand = &cli.Command{
	Name:  "start",
	Usage: "Starts a node",
	Action: func(c *cli.Context) error {
		node := c.Args().First()
		if node == "" {
			return fmt.Errorf("expected node name")
		}
		kcfg := defaults.PathToConfig("kubeconfig")
		os.Setenv("KUBECONFIG", kcfg)
		bin := defaults.PathToEmbeddedClusterBinary("kubectl")
		cmd := exec.Command(bin, "uncordon", node)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	},
}

var nodeListCommand = &cli.Command{
	Name:  "list",
	Usage: "List all nodes",
	Action: func(c *cli.Context) error {
		kcfg := defaults.PathToConfig("kubeconfig")
		os.Setenv("KUBECONFIG", kcfg)
		bin := defaults.PathToEmbeddedClusterBinary("kubectl")
		cmd := exec.Command(bin, "get", "nodes", "-o", "wide")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	},
}
