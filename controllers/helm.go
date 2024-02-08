package controllers

import (
	"fmt"
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

// detect if the charts currently installed in the cluster (currentConfigs) match the desired charts (combinedConfigs)
func detectChartDrift(combinedConfigs, currentConfigs *k0sv1beta1.HelmExtensions) (bool, error) {
	if len(currentConfigs.Charts) != len(combinedConfigs.Charts) ||
		len(currentConfigs.Repositories) != len(combinedConfigs.Repositories) {
		return true, nil
	}

	targetCharts := combinedConfigs.Charts
	chartDrift := false
	// grab the installed charts
	for _, chart := range currentConfigs.Charts {
		// check for version and values drift between installed charts and charts in the installer metadata
		chartSeen := false
		for _, targetChart := range targetCharts {
			if targetChart.Name != chart.Name {
				continue
			}
			chartSeen = true

			if targetChart.Version != chart.Version {
				chartDrift = true
			}

			valuesDiff, err := yamlDiff(targetChart.Values, chart.Values)
			if err != nil {
				return false, fmt.Errorf("failed to compare values of chart %s: %w", chart.Name, err)
			}
			if valuesDiff {
				chartDrift = true
			}
		}
		if !chartSeen { // if this chart in the cluster is not in the target spec, there is drift
			chartDrift = true
		}
	}
	return chartDrift, nil
}

// yamlDiff compares two yaml strings and returns true if they are different
func yamlDiff(a, b string) (bool, error) {
	aMap := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(a), &aMap)
	if err != nil {
		return false, fmt.Errorf("yaml A values error: %w", err)
	}

	bMap := map[string]interface{}{}
	err = yaml.Unmarshal([]byte(b), &bMap)
	if err != nil {
		return false, fmt.Errorf("yaml B values error: %w", err)
	}

	aYaml, err := yaml.Marshal(aMap)
	if err != nil {
		return false, fmt.Errorf("yaml A marshal error: %w", err)
	}

	bYaml, err := yaml.Marshal(bMap)
	if err != nil {
		return false, fmt.Errorf("yaml B marshal error: %w", err)
	}

	return string(aYaml) != string(bYaml), nil
}

// check if all charts in the combinedConfigs are installed successfully with the desired version and values
func detectChartCompletion(combinedConfigs *k0sv1beta1.HelmExtensions, installedCharts k0shelm.ChartList) ([]string, []string, error) {
	incompleteCharts := []string{}
	chartErrors := []string{}
	if combinedConfigs == nil {
		return incompleteCharts, chartErrors, nil
	}
	for _, chart := range combinedConfigs.Charts {
		diffDetected := false
		chartSeen := false
		for _, installedChart := range installedCharts.Items {
			if chart.Name == installedChart.Spec.ReleaseName {
				chartSeen = true

				valuesDiff, err := yamlDiff(chart.Values, installedChart.Spec.Values)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to compare values of chart %s: %w", chart.Name, err)
				}
				if valuesDiff {
					diffDetected = true
				}

				if installedChart.Status.Version != chart.Version {
					diffDetected = true
				}

				if installedChart.Status.Error != "" {
					chartErrors = append(chartErrors, installedChart.Status.Error)
					diffDetected = false
				}

				break
			}
		}
		if !chartSeen || diffDetected {
			incompleteCharts = append(incompleteCharts, chart.Name)
		}
	}

	return incompleteCharts, chartErrors, nil
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
