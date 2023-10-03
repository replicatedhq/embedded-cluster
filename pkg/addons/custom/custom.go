// Package custom manages the installation of custom helm charts that have been
// embedded at the end of the helmvm binary.
package custom

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
func (c *Custom) GenerateHelmConfig(ctx *cli.Context) ([]dig.Mapping, error) {

	chartConfigs := []dig.Mapping{}

	exe, err := os.Executable()
	if err != nil {
		return chartConfigs, fmt.Errorf("unable to get executable path: %w", err)
	}
	opts, err := hembed.ReadEmbedOptionsFromBinary(exe)
	if err != nil {
		return chartConfigs, fmt.Errorf("unable to read embed options: %w", err)
	} else if opts == nil {
		logrus.Warn("No embed charts found, skipping custom addons.")
		return chartConfigs, nil
	}

	for _, chart := range opts.Charts {

		chartData, err := loader.LoadArchive(chart.ChartReader())
		if err != nil {
			return chartConfigs, fmt.Errorf("unable to load chart archive: %w", err)
		}

		chartName := strings.ToLower(chartData.Name())
		chartFile := fmt.Sprintf("%s-%s.tgz", chartName, chartData.Metadata.Version)
		dstpath := filepath.Join(defaults.HelmChartSubDir(), chartFile)

		chartConfig := dig.Mapping{
			"name":      chartName,
			"namespace": c.namespace,
			"version":   chartData.Metadata.Version,
		}

		chartConfig["chartName"] = dstpath

		dst, err := os.Create(dstpath)
		if err != nil {
			logrus.Fatalf("could not write helm chart archive: %s", err)
		}

		_, err = io.Copy(dst, chart.ChartReader())
		if err != nil {
			logrus.Fatalf("could not write helm chart archive: %s", err)
		}

		chartConfigs = append(chartConfigs, chartConfig)

	}
	return chartConfigs, nil

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
