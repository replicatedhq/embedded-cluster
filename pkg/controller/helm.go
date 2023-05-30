package controller

import (
	"context"
	"fmt"
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

	"github.com/emosbaugh/helmbin/pkg/config"
	"github.com/emosbaugh/helmbin/static"
)

// Helm implement the component interface to run the Helm controller
type Helm struct {
	Options config.CLIOptions

	packages   []string
	helmConfig *action.Configuration
}

// Init initializes the Helm controller
func (k *Helm) Init(_ context.Context) error {
	env := cli.New()
	env.SetNamespace("default")
	k.helmConfig = &action.Configuration{}
	log := logrus.WithField("component", "helm")
	if err := k.helmConfig.Init(env.RESTClientGetter(), "default", "", log.Infof); err != nil {
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
		logrus.Infof("Processing Helm %s", fname)
		fp, err := static.FS().Open(fname)
		if err != nil {
			logrus.Errorf("Failed to open helm package %s: %v", fname, err)
			continue
		}
		defer func() {
			_ = fp.Close()
		}()
		chart, err := loader.LoadArchive(fp)
		if err != nil {
			logrus.Errorf("Failed to load chart %s: %v", fname, err)
			continue
		}
		fprefix := strings.TrimSuffix(fname, filepath.Ext(fname))
		yamlfile := fmt.Sprintf("%s.yaml", fprefix)
		content, err := static.FS().ReadFile(yamlfile)
		if err != nil {
			logrus.Errorf("Failed to read values file %s: %v", yamlfile, err)
			continue
		}
		values := map[string]interface{}{}
		if err := yaml.Unmarshal(content, &values); err != nil {
			logrus.Errorf("Failed to unmarshal values file %s: %v", yamlfile, err)
			continue
		}
		if err := k.applyChart(ctx, chart, nil); err != nil {
			logrus.Errorf("Failed to apply chart %s: %v", fname, err)
			continue
		}
	}
}

func (k *Helm) applyChart(ctx context.Context, chart *chart.Chart, values map[string]interface{}) error {
	installed, err := k.installedRelease(ctx, chart.Name())
	if err != nil {
		return fmt.Errorf("failed to check if release %s is installed: %w", chart.Name(), err)
	}
	if installed == nil {
		logrus.Infof("Helm %s hasn't been installed yet, installing it.", chart.Name())
		act := action.NewInstall(k.helmConfig)
		act.Namespace = "default"
		act.ReleaseName = chart.Name()
		if _, err := act.RunWithContext(ctx, chart, values); err != nil {
			return fmt.Errorf("failed to install chart %s: %w", chart.Name(), err)
		}
		return nil
	}
	logrus.Infof("Helm %s is already installed, applying changes (possible upgrade).", chart.Name())
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
