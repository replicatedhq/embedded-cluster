package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
)

var configCommand = &cli.Command{
	Name:  "config",
	Usage: "Dumps generated config to stdout",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:   "overrides",
			Usage:  "File with an EmbeddedClusterConfig object to override the default configuration",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:  "no-prompt",
			Usage: "Do not prompt user when it is not necessary",
			Value: false,
		},
	},
	Action: func(c *cli.Context) error {
		multi := false

		cfg, err := config.RenderClusterConfig(c.Context, multi)
		if err != nil {
			return fmt.Errorf("unable to render config: %w", err)
		}
		opts := []addons.Option{}
		if c.Bool("no-prompt") {
			opts = append(opts, addons.WithoutPrompt())
		}
		for _, addon := range c.StringSlice("disable-addon") {
			opts = append(opts, addons.WithoutAddon(addon))
		}
		if err := config.UpdateHelmConfigs(cfg, opts...); err != nil {
			return fmt.Errorf("unable to update helm configs: %w", err)
		}
		if err := applyUnsupportedOverrides(c, cfg); err != nil {
			return fmt.Errorf("unable to apply unsupported overrides: %w", err)
		}
		if err := yaml.NewEncoder(os.Stdout).Encode(cfg); err != nil {
			return fmt.Errorf("unable to write config file: %w", err)
		}

		return nil
	},
}
