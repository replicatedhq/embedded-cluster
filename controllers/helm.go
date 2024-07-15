package controllers

import (
	"fmt"

	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"sigs.k8s.io/yaml"
)

// detect if the charts currently installed in the cluster (currentConfigs) match the desired charts (combinedConfigs)
func detectChartDrift(combinedConfigs, currentConfigs *k0sv1beta1.HelmExtensions) (bool, []string, error) {
	chartDrift := false
	driftMap := map[string]struct{}{}
	if len(currentConfigs.Repositories) != len(combinedConfigs.Repositories) {
		chartDrift = true
		driftMap["repositories"] = struct{}{}
	}

	targetCharts := combinedConfigs.Charts
	// grab the desired charts
	for _, targetChart := range targetCharts {
		// check for version and values drift between installed charts and desired charts
		chartSeen := false
		for _, chart := range currentConfigs.Charts {
			if targetChart.Name != chart.Name {
				continue
			}
			chartSeen = true

			if targetChart.Version != chart.Version {
				chartDrift = true
				driftMap[chart.Name] = struct{}{}
			}

			valuesDiff, err := yamlDiff(targetChart.Values, chart.Values)
			if err != nil {
				return false, nil, fmt.Errorf("failed to compare values of chart %s: %w", chart.Name, err)
			}
			if valuesDiff {
				chartDrift = true
				driftMap[chart.Name] = struct{}{}
			}
		}
		if !chartSeen { // if this chart in the spec is not in the cluster, there is drift
			chartDrift = true
			driftMap[targetChart.Name] = struct{}{}
		}
	}

	// flatten map to []string
	driftSlice := []string{}
	for k := range driftMap {
		driftSlice = append(driftSlice, k)
	}

	return chartDrift, driftSlice, nil
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
func detectChartCompletion(existingHelm *k0sv1beta1.HelmExtensions, installedCharts k0shelm.ChartList) ([]string, []string, error) {
	incompleteCharts := []string{}
	chartErrors := []string{}
	if existingHelm == nil {
		return incompleteCharts, chartErrors, nil
	}
	for _, chart := range existingHelm.Charts {
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

				// if the spec's HashValues does not match the status's ValuesHash, the chart is currently being applied with the new values
				if installedChart.Spec.HashValues() != installedChart.Status.ValuesHash {
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
