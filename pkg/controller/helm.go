package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"

	"github.com/replicatedhq/helmbin/pkg/config"
	"github.com/replicatedhq/helmbin/static"
)

// Helm implement the component interface to run the Helm controller
type Helm struct {
	Options config.CLIOptions

	packages   []string
	helmConfig *action.Configuration
	log        logrus.FieldLogger
}

// Init initializes the Helm controller
func (k *Helm) Init(_ context.Context) error {
	env := cli.New()
	env.SetNamespace("default")
	k.helmConfig = &action.Configuration{}
	k.log = logrus.WithField("component", "helm")
	if err := k.helmConfig.Init(env.RESTClientGetter(), "default", "", k.log.Infof); err != nil {
		return fmt.Errorf("failed to init helm configuration: %w", err)
	}
	packages, err := static.FS().ReadDir("helm")
	if err != nil {
		return fmt.Errorf("failed to read helm directory: %w", err)
	}
	for _, entry := range packages {
		if !strings.HasSuffix(entry.Name(), ".tgz") {
			continue
		}
		fpath := fmt.Sprintf("helm/%s", entry.Name())
		k.packages = append(k.packages, fpath)
	}
	return nil
}

// Start starts the Helm controller
func (k *Helm) Start(ctx context.Context) error {
	go k.apply(ctx)
	return nil
}

func (k *Helm) apply(ctx context.Context) {
	for _, fname := range k.packages {
		err := k.applyOne(ctx, fname)
		if err != nil {
			k.log.Errorf("Failed to apply chart %s: %v", fname, err)
		}
	}
}

func (k *Helm) applyOne(ctx context.Context, fname string) error {
	fprefix := strings.TrimSuffix(fname, filepath.Ext(fname))
	k.log.Infof("Processing chart %s", fname)
	fp, err := static.FS().Open(fname)
	if err != nil {
		return fmt.Errorf("failed to open chart archive: %w", err)
	}
	defer func() {
		_ = fp.Close()
	}()
	chart, err := loader.LoadArchive(fp)
	if err != nil {
		return fmt.Errorf("failed to load chart archive: %w", err)
	}
	yamlfile := fmt.Sprintf("%s.yaml", fprefix)
	content, err := static.FS().ReadFile(yamlfile)
	values := map[string]interface{}{}
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read values file %s: %w", yamlfile, err)
		}
	} else {
		k.log.Infof("Found chart values %s", yamlfile)
		if err := yaml.Unmarshal(content, &values); err != nil {
			return fmt.Errorf("failed to unmarshal values file: %w", err)
		}
	}
	return k.applyChart(ctx, chart, values)
}

func (k *Helm) applyChart(ctx context.Context, chart *chart.Chart, values map[string]interface{}) error {
	installed, err := k.installedRelease(ctx, chart.Name())
	if err != nil {
		return fmt.Errorf("failed to check if release %s is installed: %w", chart.Name(), err)
	}
	if installed == nil {
		k.log.Infof("Helm %s hasn't been installed yet, installing it.", chart.Name())
		act := action.NewInstall(k.helmConfig)
		act.Namespace = "default"
		act.ReleaseName = chart.Name()
		if _, err := act.RunWithContext(ctx, chart, values); err != nil {
			return fmt.Errorf("failed to install chart %s: %w", chart.Name(), err)
		}
		return nil
	}
	k.log.Infof("Helm %s is already installed, applying changes (possible upgrade).", chart.Name())
	curver := fmt.Sprintf("v%s", installed.Chart.Metadata.Version)
	newver := fmt.Sprintf("v%s", chart.Metadata.Version)
	if out := semver.Compare(curver, newver); out > 0 {
		return fmt.Errorf("%s %s installed, unable to downgrade to %s", chart.Name(), curver, newver)
	}
	act := action.NewUpgrade(k.helmConfig)
	act.Namespace = "default"
	if _, err := act.RunWithContext(ctx, chart.Name(), chart, values); err != nil {
		return fmt.Errorf("failed to upgrade chart %s: %w", chart.Name(), err)
	}
	return nil
}

func (k *Helm) installedRelease(_ context.Context, _ string) (*release.Release, error) {
	list := action.NewList(k.helmConfig)
	list.StateMask = action.ListDeployed
	releases, err := list.Run()
	if err != nil {
		return nil, fmt.Errorf("unable to list installed releases: %w", err)
	}
	if len(releases) == 0 {
		return nil, nil
	}
	return releases[0], nil
}

// Stop stops the Helm controller
func (k *Helm) Stop() error {
	return nil
}

// Ready is the health-check interface
func (k *Helm) Ready() error {
	// TODO
	return nil
}
