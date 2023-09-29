// Package openebs manages the OpenEBS storage provisioner helm chart
// installation or upgrade in the cluster.
package openebs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/k0sproject/dig"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/urfave/cli/v2"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"

	"github.com/replicatedhq/helmvm/pkg/addons/openebs/charts"
	"github.com/replicatedhq/helmvm/pkg/defaults"
)

const (
	releaseName = "openebs"
	appVersion  = "3.7.0"
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

func (o *OpenEBS) GetChartFileName() string {
	return fmt.Sprintf("openebs-%s.tgz", appVersion)
}

func (o *OpenEBS) GenerateHelmConfig(ctx *cli.Context) (dig.Mapping, error) {
	chartConfig := dig.Mapping{
		"name":      releaseName,
		"namespace": o.namespace,
		"version":   appVersion,
	}

	chartConfig["chartName"] = filepath.Join(defaults.HelmChartSubDir(), o.GetChartFileName())

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return chartConfig, err
	}
	chartConfig["values"] = string(valuesStringData)

	return chartConfig, nil
}

func (o *OpenEBS) WtriteChartFile() error {
	chartfile := o.GetChartFileName()

	src, err := charts.FS.Open(chartfile)
	if err != nil {
		return fmt.Errorf("could not load chart file: %w", err)
	}

	dstpath := filepath.Join(defaults.HelmChartSubDir(), chartfile)
	dst, err := os.Create(dstpath)
	if err != nil {
		return fmt.Errorf("could not write helm chart archive: %w", err)
	}

	io.Copy(dst, src)
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
	return &OpenEBS{namespace: namespace, logger: logger}, nil
}
