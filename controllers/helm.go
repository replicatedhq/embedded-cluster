package controllers

import (
	"fmt"
	"reflect"

	"github.com/k0sproject/dig"
	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
	"sigs.k8s.io/yaml"

	"github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/release"
)

// MergeValues takes two helm values in the form of dig.Mapping{} and a list of values (in jsonpath notation) to not override
// and combines the values. it returns the resultant yaml string
func MergeValues(oldValues, newValues string, protectedValues []string) (string, error) {

	newValuesMap := dig.Mapping{}
	if err := yaml.Unmarshal([]byte(newValues), &newValuesMap); err != nil {
		return "", fmt.Errorf("failed to unmarshal new chart values: %w", err)
	}

	// merge the known fields from the current chart values to the new chart values
	for _, path := range protectedValues {
		x, err := jp.ParseString(path)
		if err != nil {
			return "", fmt.Errorf("failed to parse json path: %w", err)
		}

		valuesJson, err := yaml.YAMLToJSON([]byte(oldValues))
		if err != nil {
			return "", fmt.Errorf("failed to convert yaml to json: %w", err)
		}

		obj, err := oj.ParseString(string(valuesJson))
		if err != nil {
			return "", fmt.Errorf("failed to parse json: %w", err)
		}

		value := x.Get(obj)

		// if the value is empty, skip it
		if len(value) < 1 {
			continue
		}

		err = x.Set(newValuesMap, value[0])
		if err != nil {
			return "", fmt.Errorf("failed to set json path: %w", err)
		}
	}

	newValuesYaml, err := yaml.Marshal(newValuesMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal new chart values: %w", err)
	}
	return string(newValuesYaml), nil

}

// merge the default helm charts and repositories (from meta.Configs) with vendor helm charts (from in.Spec.Config.Extensions.Helm)
func mergeHelmConfigs(meta *release.Meta, in *v1beta1.Installation) *k0sv1beta1.HelmExtensions {
	// merge default helm charts (from meta.Configs) with vendor helm charts (from in.Spec.Config.Extensions.Helm)
	combinedConfigs := &k0sv1beta1.HelmExtensions{ConcurrencyLevel: 1}
	if meta != nil && meta.Configs != nil {
		combinedConfigs = meta.Configs
	}
	if in != nil && in.Spec.Config != nil && in.Spec.Config.Extensions.Helm != nil {
		// set the concurrency level to the minimum of our default and the user provided value
		if in.Spec.Config.Extensions.Helm.ConcurrencyLevel > 0 {
			combinedConfigs.ConcurrencyLevel = min(in.Spec.Config.Extensions.Helm.ConcurrencyLevel, combinedConfigs.ConcurrencyLevel)
		}

		// append the user provided charts to the default charts
		combinedConfigs.Charts = append(combinedConfigs.Charts, in.Spec.Config.Extensions.Helm.Charts...)
		// append the user provided repositories to the default repositories
		combinedConfigs.Repositories = append(combinedConfigs.Repositories, in.Spec.Config.Extensions.Helm.Repositories...)
	}
	return combinedConfigs
}

// detect if the charts currently installed in the cluster (installedCharts) match the desired charts (combinedConfigs)
// also detect if any of the charts installed in the cluster contain error messages
func detectChartDrift(combinedConfigs *k0sv1beta1.HelmExtensions, installedCharts k0shelm.ChartList) ([]string, bool, error) {
	targetCharts := combinedConfigs.Charts
	chartErrors := []string{}
	chartDrift := len(installedCharts.Items) != len(targetCharts)
	// grab the installed charts
	for _, chart := range installedCharts.Items {
		// extract any errors from installed charts
		if chart.Status.Error != "" {
			chartErrors = append(chartErrors, chart.Status.Error)
		}
		// check for version and values drift between installed charts and charts in the installer metadata
		chartSeen := false
		for _, targetChart := range targetCharts {
			if targetChart.Name != chart.Spec.ReleaseName {
				continue
			}
			chartSeen = true

			checkVersion := chart.Status.Version
			if checkVersion == "" { // status will not contain the version if there is an error
				checkVersion = chart.Spec.Version
			}
			if targetChart.Version != checkVersion {
				chartDrift = true
			}
			targetValuesMap := map[string]interface{}{}
			err := yaml.Unmarshal([]byte(targetChart.Values), &targetValuesMap)
			if err != nil {
				return nil, false, fmt.Errorf("target chart %s values error: %w", targetChart.Name, err)
			}

			currentValuesMap := map[string]interface{}{}
			err = yaml.Unmarshal([]byte(chart.Spec.Values), &currentValuesMap)
			if err != nil {
				return nil, false, fmt.Errorf("existing chart %s values error: %w", chart.Spec.ReleaseName, err)
			}

			if !reflect.DeepEqual(targetValuesMap, currentValuesMap) {
				chartDrift = true
			}

		}
		if !chartSeen { // if this chart in the cluster is not in the target spec, there is drift
			chartDrift = true
		}
	}
	return chartErrors, chartDrift, nil
}

// merge the helmcharts in the cluster with the charts we desire to be in the cluster
// if the chart is already in the cluster, merge the values
func generateDesiredCharts(meta *release.Meta, clusterconfig k0sv1beta1.ClusterConfig, combinedConfigs *k0sv1beta1.HelmExtensions) ([]k0sv1beta1.Chart, error) {
	// get the protected values from the release metadata
	protectedValues := map[string][]string{}
	if meta != nil && meta.Protected != nil {
		protectedValues = meta.Protected
	}

	// TODO - apply unsupported override from installation config
	finalConfigs := map[string]k0sv1beta1.Chart{}
	// include charts in the final spec that are already in the cluster (with merged values)
	for _, chart := range clusterconfig.Spec.Extensions.Helm.Charts {
		for _, newChart := range combinedConfigs.Charts {
			// check if we can skip this chart
			_, ok := protectedValues[chart.Name]
			if chart.Name != newChart.Name || !ok {
				continue
			}
			// if we have known fields, we need to merge them forward
			newValuesYaml, err := MergeValues(chart.Values, newChart.Values, protectedValues[chart.Name])
			if err != nil {
				return nil, fmt.Errorf("failed to merge chart values: %w", err)
			}
			newChart.Values = newValuesYaml
			finalConfigs[newChart.Name] = newChart
			break
		}
	}
	// include new charts in the final spec that are not yet in the cluster
	for _, newChart := range combinedConfigs.Charts {
		if _, ok := finalConfigs[newChart.Name]; !ok {
			finalConfigs[newChart.Name] = newChart
		}
	}

	// flatten chart map
	finalChartList := []k0sv1beta1.Chart{}
	for _, chart := range finalConfigs {
		finalChartList = append(finalChartList, chart)
	}
	return finalChartList, nil
}

// shouldNotUpdateClusterConfig returns true if there are charts within the clusterConfig that have not yet been applied
// to the cluster. if we update the cluster config while charts are still being applied, k0s may attempt to apply the
// same chart twice in parallel, which causes an error
// 'can't install loadedChart `CHART_NAME_HERE`: cannot re-use a name that is still in use'
// returns the list of charts that are still being applied
func shouldNotUpdateClusterConfig(configCharts *k0sv1beta1.HelmExtensions, charts k0shelm.ChartList) []string {
	pendingCharts := []string{}
	if configCharts == nil {
		return pendingCharts
	}

	for _, specChart := range configCharts.Charts {
		foundChart := false
		for _, clusterChart := range charts.Items {
			if specChart.Name == clusterChart.Spec.ReleaseName {
				foundChart = true

				if clusterChart.Status.ReleaseName == "" {
					// this chart has not yet been applied to the cluster, otherwise the ReleaseName would be set
					pendingCharts = append(pendingCharts, specChart.Name)
				}

				break
			}
		}

		if !foundChart {
			// this chart is present in the spec, but not the cluster, and thus is still being applied
			pendingCharts = append(pendingCharts, specChart.Name)
		}
	}

	return pendingCharts
}
