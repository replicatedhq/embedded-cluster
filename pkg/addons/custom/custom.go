// Package custom manages the installation of custom helm charts that have been
// embedded at the end of the helmvm binary.
package custom

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"

	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/hembed"
)

type Custom struct {
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

// GenerateHelmConfig generates the helm config for all the embedded charts.
// and writes the charts to the disk.
func (c *Custom) GenerateHelmConfig() ([]v1beta1.Chart, []v1beta1.Repository, error) {

	chartConfigs := []v1beta1.Chart{}

	exe, err := os.Executable()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get executable path: %w", err)
	}
	opts, err := hembed.ReadEmbedOptionsFromBinary(exe)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read embed options: %w", err)
	} else if opts == nil {
		return nil, nil, nil
	}

	for _, chart := range opts.Charts {

		chartData, err := loader.LoadArchive(chart.ChartReader())
		if err != nil {
			return nil, nil, fmt.Errorf("unable to load chart archive: %w", err)
		}

		if c.chartHasBeenDisabled(chartData) {
			logrus.Infof("skipping disabled chart %s", chartData.Name())
			continue
		}

		chartName := strings.ToLower(chartData.Name())
		dstpath := defaults.PathToHelmChart(chartName, chartData.Metadata.Version)

		chartConfig := v1beta1.Chart{
			Name:      chartName,
			Version:   chartData.Metadata.Version,
			TargetNS:  c.namespace,
			ChartName: dstpath,
			Values:    chart.Values,
		}

		reader := chart.ChartReader()
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to read helm chart archive: %w", err)
		}

		err = os.WriteFile(dstpath, data, 0644)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to write helm chart archive: %w", err)
		}

		chartConfigs = append(chartConfigs, chartConfig)

	}
	return chartConfigs, nil, nil

}

func (c *Custom) chartHasBeenDisabled(chart *chart.Chart) bool {
	cname := strings.ToLower(chart.Name())
	_, disabledAddons := c.disabledAddons[cname]
	return disabledAddons
}

func New(namespace string, disabledAddons map[string]bool) (*Custom, error) {
	return &Custom{
		namespace:      namespace,
		disabledAddons: disabledAddons,
	}, nil
}
