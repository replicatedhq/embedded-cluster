// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"

	"github.com/replicatedhq/helmvm/pkg/addons/adminconsole/charts"
	"github.com/replicatedhq/helmvm/pkg/prompts"
)

const (
	releaseName = "adminconsole"
)

var helmValues = map[string]interface{}{
	"password":      "password",
	"minimalRBAC":   false,
	"isHelmManaged": false,
	"service": map[string]interface{}{
		"type":     "NodePort",
		"nodePort": 30000,
	},
}

type AdminConsole struct {
	customization AdminConsoleCustomization
	config        *action.Configuration
	logger        action.DebugLog
	namespace     string
	useprompt     bool
}

func (a *AdminConsole) askPassword() (string, error) {
	if !a.useprompt {
		logrus.Warnf("Admin Console password set to: password")
		return "password", nil
	}
	return prompts.New().Password("Enter a new Admin Console password:"), nil
}

func (a *AdminConsole) Version() (map[string]string, error) {
	latest, err := a.Latest()
	if err != nil {
		return nil, fmt.Errorf("unable to get latest version: %w", err)
	}
	return map[string]string{"AdminConsole": latest}, nil
}

func (a *AdminConsole) Apply(ctx context.Context) error {
	version, err := a.Latest()
	if err != nil {
		return fmt.Errorf("unable to get latest Admin Console version: %w", err)
	}
	if !semver.IsValid(version) {
		return fmt.Errorf("unable to parse version %s", version)
	}

	fname := fmt.Sprintf("adminconsole-%s.tgz", strings.TrimPrefix(version, "v"))
	hfp, err := charts.FS.Open(fname)
	if err != nil {
		return fmt.Errorf("unable to find version %s: %w", version, err)
	}
	defer hfp.Close()

	hchart, err := loader.LoadArchive(hfp)
	if err != nil {
		return fmt.Errorf("unable to load chart: %w", err)
	}

	release, err := a.installedRelease(ctx)
	if err != nil {
		return fmt.Errorf("unable to list adminconsole releases: %w", err)
	}

	if release == nil {
		a.logger("Admin Console hasn't been installed yet, installing it.")
		pass, err := a.askPassword()
		if err != nil {
			return fmt.Errorf("unable to ask for password: %w", err)
		}
		helmValues["password"] = pass
		act := action.NewInstall(a.config)
		act.Namespace = a.namespace
		act.ReleaseName = releaseName
		act.CreateNamespace = true
		if _, err := act.RunWithContext(ctx, hchart, helmValues); err != nil {
			return fmt.Errorf("unable to install chart: %w", err)
		}
		return a.customization.apply(ctx)
	}

	a.logger("Admin Console already installed on the cluster, checking version.")
	installedVersion := fmt.Sprintf("v%s", release.Chart.Metadata.Version)
	if out := semver.Compare(installedVersion, version); out > 0 {
		return fmt.Errorf("unable to downgrade from %s to %s", installedVersion, version)
	}

	a.logger("Updating Admin Console from %s to %s", installedVersion, version)
	act := action.NewUpgrade(a.config)
	act.Namespace = a.namespace
	if _, err := act.RunWithContext(ctx, releaseName, hchart, helmValues); err != nil {
		return fmt.Errorf("unable to upgrade chart: %w", err)
	}
	return a.customization.apply(ctx)
}

func (a *AdminConsole) Latest() (string, error) {
	a.logger("Finding Latest Admin Console addon version")
	files, err := charts.FS.ReadDir(".")
	if err != nil {
		return "", fmt.Errorf("unable to read charts directory: %w", err)
	}
	var latest string
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".tgz") {
			continue
		}
		trimmed := strings.TrimSuffix(file.Name(), ".tgz")
		slices := strings.Split(trimmed, "-")
		if len(slices) != 2 {
			return "", fmt.Errorf("invalid file name found: %s", file.Name())
		}
		currentV := fmt.Sprintf("v%s", slices[1])
		if latest == "" {
			latest = currentV
			continue
		}
		if semver.Compare(latest, currentV) < 0 {
			latest = currentV
		}
	}
	a.logger("Latest Admin Console version found: %s", latest)
	return latest, nil
}

func (a *AdminConsole) installedRelease(ctx context.Context) (*release.Release, error) {
	list := action.NewList(a.config)
	list.StateMask = action.ListDeployed
	list.Filter = releaseName
	releases, err := list.Run()
	if err != nil {
		return nil, fmt.Errorf("unable to list installed releases: %w", err)
	}
	if len(releases) == 0 {
		return nil, nil
	}
	return releases[0], nil
}

func New(ns string, useprompt bool, log action.DebugLog) (*AdminConsole, error) {
	env := cli.New()
	env.SetNamespace(ns)
	config := &action.Configuration{}
	if err := config.Init(env.RESTClientGetter(), ns, "", log); err != nil {
		return nil, fmt.Errorf("unable to init configuration: %w", err)
	}
	return &AdminConsole{
		namespace:     ns,
		config:        config,
		logger:        log,
		useprompt:     useprompt,
		customization: AdminConsoleCustomization{},
	}, nil
}
