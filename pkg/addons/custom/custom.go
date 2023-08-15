// Package custom manages the installation of custom helm charts that have been
// embedded at the end of the helmvm binary.
package custom

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/mod/semver"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"sigs.k8s.io/yaml"

	"github.com/replicatedhq/helmvm/pkg/hembed"
)

type Custom struct {
	config    *action.Configuration
	logger    action.DebugLog
	namespace string
}

func (c *Custom) Apply(ctx context.Context) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to get executable path: %w", err)
	}
	opts, err := hembed.ReadEmbedOptionsFromBinary(exe)
	if err != nil {
		return fmt.Errorf("unable to read embed options: %w", err)
	} else if opts == nil {
		c.logger("No embed charts found, skipping custom addons.")
		return nil
	}
	for _, chart := range opts.Charts {
		if err := c.applyOne(ctx, chart); err != nil {
			return fmt.Errorf("unable to apply chart: %w", err)
		}
	}
	return nil
}

func (c *Custom) applyOne(ctx context.Context, ochart hembed.HelmChart) error {
	chart, err := loader.LoadArchive(ochart.ChartReader())
	if err != nil {
		return fmt.Errorf("unable to load chart archive: %w", err)
	}
	var values map[string]interface{}
	if len(ochart.Values) > 0 {
		values = make(map[string]interface{})
		if err := yaml.Unmarshal([]byte(ochart.Values), &values); err != nil {
			return fmt.Errorf("unable to unmarshal values: %w", err)
		}
	}
	return c.applyChart(ctx, chart, values)
}

func (c *Custom) applyChart(ctx context.Context, chart *chart.Chart, values map[string]interface{}) error {
	installed, err := c.installedRelease(chart.Name())
	if err != nil {
		return fmt.Errorf("unable to check if release %s is installed: %w", chart.Name(), err)
	}
	if installed == nil {
		c.logger("Custom %s hasn't been installed yet, installing it.", chart.Name())
		act := action.NewInstall(c.config)
		act.Namespace = "helmvm"
		act.ReleaseName = chart.Name()
		act.CreateNamespace = true
		if _, err := act.RunWithContext(ctx, chart, values); err != nil {
			return fmt.Errorf("unable to install chart %s: %w", chart.Name(), err)
		}
		return nil
	}
	c.logger("Custom %s is already installed, applying changes.", chart.Name())
	curver := fmt.Sprintf("v%s", installed.Chart.Metadata.Version)
	newver := fmt.Sprintf("v%s", chart.Metadata.Version)
	if out := semver.Compare(curver, newver); out > 0 {
		return fmt.Errorf("%s %s installed, unable to downgrade to %s", chart.Name(), curver, newver)
	}
	act := action.NewUpgrade(c.config)
	act.Namespace = "helmvm"
	if _, err := act.RunWithContext(ctx, chart.Name(), chart, values); err != nil {
		return fmt.Errorf("unable to upgrade chart %s: %w", chart.Name(), err)
	}
	return nil
}

func (c *Custom) installedRelease(name string) (*release.Release, error) {
	list := action.NewList(c.config)
	list.StateMask = action.ListDeployed
	list.Filter = name
	releases, err := list.Run()
	if err != nil {
		return nil, fmt.Errorf("unable to list installed releases: %w", err)
	}
	if len(releases) == 0 {
		return nil, nil
	}
	return releases[0], nil
}

func New(namespace string, logger action.DebugLog) (*Custom, error) {
	env := cli.New()
	env.SetNamespace(namespace)
	config := &action.Configuration{}
	if err := config.Init(env.RESTClientGetter(), namespace, "", logger); err != nil {
		return nil, fmt.Errorf("unable to init configuration: %w", err)
	}
	return &Custom{namespace: namespace, config: config, logger: logger}, nil
}
