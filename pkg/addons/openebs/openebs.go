// Package openebs manages the OpenEBS storage provisioner helm chart
// installation or upgrade in the cluster.
package openebs

import (
	"context"
	"fmt"
	"strings"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"golang.org/x/mod/semver"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"

	"github.com/replicatedhq/helmvm/pkg/addons/openebs/charts"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
)

const (
	releaseName = "openebs"
)

var helmValues = map[string]interface{}{
	"ndmOperator": map[string]interface{}{
		"enabled": false,
	},
	"ndm": map[string]interface{}{
		"enabled": false,
	},
	"localprovisioner": map[string]interface{}{
		"hostpathClass": map[string]interface{}{
			"isDefaultClass": true,
		},
	},
}

type OpenEBS struct {
	config    *action.Configuration
	logger    action.DebugLog
	namespace string
}

func (o *OpenEBS) Version() (map[string]string, error) {
	latest, err := o.latest()
	if err != nil {
		return nil, fmt.Errorf("unable to get latest version: %w", err)
	}
	return map[string]string{"OpenEBS": latest}, nil
}

// HostPreflight returns the host preflight objects found inside the OpenEBS
// Helm Chart, this is empty as there is no host preflight on there.
func (o *OpenEBS) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
}

func (o *OpenEBS) Apply(ctx context.Context) error {
	loading := pb.Start()
	loading.Infof("Applying OpenEBS addon")
	defer loading.Close()
	version, err := o.latest()
	if err != nil {
		return fmt.Errorf("unable to get latest version: %w", err)
	}
	if !semver.IsValid(version) {
		return fmt.Errorf("unable to parse version %s", version)
	}
	fname := fmt.Sprintf("openebs-%s.tgz", strings.TrimPrefix(version, "v"))
	hfp, err := charts.FS.Open(fname)
	if err != nil {
		return fmt.Errorf("unable to find version %s: %w", version, err)
	}
	defer hfp.Close()

	hchart, err := loader.LoadArchive(hfp)
	if err != nil {
		return fmt.Errorf("unable to load chart: %w", err)
	}

	release, err := o.installedRelease(ctx)
	if err != nil {
		return fmt.Errorf("unable to list openebs releases: %w", err)
	}

	if release == nil {
		o.logger("OpenEBS hasn't been installed yet, installing it.")
		act := action.NewInstall(o.config)
		act.Namespace = o.namespace
		act.ReleaseName = releaseName
		act.CreateNamespace = true
		if _, err := act.RunWithContext(ctx, hchart, helmValues); err != nil {
			return fmt.Errorf("unable to install chart: %w", err)
		}
		return nil
	}

	o.logger("OpenEBS already installed on the cluster, checking version.")
	installedVersion := fmt.Sprintf("v%s", release.Chart.Metadata.Version)
	if out := semver.Compare(installedVersion, version); out > 0 {
		return fmt.Errorf("unable to downgrade from %s to %s", installedVersion, version)
	}

	o.logger("Updating OpenEBS from %s to %s", installedVersion, version)
	act := action.NewUpgrade(o.config)
	act.Namespace = o.namespace
	if _, err := act.RunWithContext(ctx, releaseName, hchart, helmValues); err != nil {
		return fmt.Errorf("unable to upgrade chart: %w", err)
	}
	return nil
}

func (o *OpenEBS) latest() (string, error) {
	o.logger("Finding latest OpenEBS addon version")
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
	o.logger("Latest OpenEBS version found: %s", latest)
	return latest, nil
}

func (o *OpenEBS) installedRelease(ctx context.Context) (*release.Release, error) {
	list := action.NewList(o.config)
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

func New(namespace string, logger action.DebugLog) (*OpenEBS, error) {
	env := cli.New()
	env.SetNamespace(namespace)
	config := &action.Configuration{}
	if err := config.Init(env.RESTClientGetter(), namespace, "", logger); err != nil {
		return nil, fmt.Errorf("unable to init configuration: %w", err)
	}
	return &OpenEBS{namespace: namespace, config: config, logger: logger}, nil
}
