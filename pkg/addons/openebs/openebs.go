// Package openebs manages the OpenEBS storage provisioner helm chart
// installation or upgrade in the cluster.
package openebs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/k0sproject/dig"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v2"

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

func (o *OpenEBS) GenerateHelmConfig(ctx *cli.Context) ([]dig.Mapping, error) {

	chartConfigs := []dig.Mapping{}

	chartConfig := dig.Mapping{
		"name":      releaseName,
		"namespace": o.namespace,
		"version":   appVersion,
	}

	chartConfig["chartName"] = filepath.Join(defaults.HelmChartSubDir(), o.GetChartFileName())

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return chartConfigs, err
	}
	chartConfig["values"] = string(valuesStringData)

	chartConfigs = append(chartConfigs, chartConfig)

	err = o.WriteChartFile()
	if err != nil {
		logrus.Fatalf("could not write chart file: %s", err)
	}

	return chartConfigs, nil
}

func (o *OpenEBS) WriteChartFile() error {
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
	logrus.Info("Finding latest OpenEBS addon version")
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
	logrus.Infof("Latest OpenEBS version found: %s", latest)
	return latest, nil
}

func New(namespace string) (*OpenEBS, error) {
	return &OpenEBS{namespace: namespace}, nil
}
