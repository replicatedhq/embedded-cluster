// Package custom manages the installation of custom helm charts that have been
// embedded at the end of the helmvm binary.
package custom

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"golang.org/x/mod/semver"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"sigs.k8s.io/yaml"

	"github.com/replicatedhq/helmvm/pkg/hembed"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
)

type Custom struct {
	config         *action.Configuration
	logger         action.DebugLog
	namespace      string
	disabledAddons map[string]bool
}

func (c *Custom) Version() (map[string]string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("unable to get executable path: %w", err)
	}
	opts, err := hembed.ReadEmbedOptionsFromBinary(exe)
	if err != nil {
		return nil, fmt.Errorf("unable to read embed options: %w", err)
	} else if opts == nil {
		return nil, nil
	}
	infos := make(map[string]string)
	for _, raw := range opts.Charts {
		chart, err := loader.LoadArchive(raw.ChartReader())
		if err != nil {
			return nil, fmt.Errorf("unable to load chart archive: %w", err)
		}
		infos[chart.Name()] = chart.Metadata.Version
	}
	return infos, nil
}

// HostPreflight returns the host preflight objects found inside all the embedded
// Helm Charts. These host preflights must be merged into a single one. XXX We have
// to implement this yet.
func (c *Custom) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
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
	if c.chartHasBeenDisabled(chart) {
		c.logger("Skipping disabled addon %s", chart.Name())
		return nil
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

func (c *Custom) chartHasBeenDisabled(chart *chart.Chart) bool {
	cname := strings.ToLower(chart.Name())
	_, disabledAddons := c.disabledAddons[cname]
	return disabledAddons
}

func (c *Custom) applyChart(ctx context.Context, chart *chart.Chart, values map[string]interface{}) error {
	loading := pb.Start()
	loading.Infof("Applying %s addon", chart.Name())
	defer loading.Close()
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

func New(namespace string, logger action.DebugLog, disabledAddons map[string]bool) (*Custom, error) {
	env := cli.New()
	env.SetNamespace(namespace)
	config := &action.Configuration{}
	if err := config.Init(env.RESTClientGetter(), namespace, "", logger); err != nil {
		return nil, fmt.Errorf("unable to init configuration: %w", err)
	}
	return &Custom{
		namespace:      namespace,
		config:         config,
		logger:         logger,
		disabledAddons: disabledAddons,
	}, nil
}
