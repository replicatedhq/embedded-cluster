// Package openebs manages the OpenEBS storage provisioner helm chart
// installation or upgrade in the cluster.
package openebs

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v2"

	"github.com/replicatedhq/helmvm/pkg/addons/openebs/charts"
	"github.com/replicatedhq/helmvm/pkg/defaults"
)

const (
	releaseName = "openebs"
)

var Version = "v0.0.0"

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
	return map[string]string{"OpenEBS": "v" + latest}, nil
}

// HostPreflight returns the host preflight objects found inside the OpenEBS
// Helm Chart, this is empty as there is no host preflight on there.
func (o *OpenEBS) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
}

func (o *OpenEBS) GenerateHelmConfig() ([]v1beta1.Chart, []v1beta1.Repository, error) {
	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: "openebs/openebs",
		Version:   Version,
		TargetNS:  o.namespace,
	}

	repositoryConfig := v1beta1.Repository{
		Name: "openebs",
		URL:  "https://openebs.github.io/charts",
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	/*err = o.WriteChartFile(Version)
	if err != nil {
		logrus.Fatalf("could not write chart file: %s", err)
	}*/

	return []v1beta1.Chart{chartConfig}, []v1beta1.Repository{repositoryConfig}, nil
}

func (o *OpenEBS) WriteChartFile(version string) error {

	chartfile := fmt.Sprintf("%s-%s.tgz", releaseName, version)

	src, err := charts.FS.Open(chartfile)
	if err != nil {
		return fmt.Errorf("unable to open helm chart archive: %w", err)
	}

	dstpath := defaults.PathToHelmChart(releaseName, version)

	sourceFileByte, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("unable to read helm chart archive: %w", err)
	}

	err = os.WriteFile(dstpath, sourceFileByte, 0644)
	if err != nil {
		return fmt.Errorf("unable to write helm chart archive: %w", err)
	}

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
		currentV := slices[1]
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
