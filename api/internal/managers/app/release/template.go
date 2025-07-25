package release

import (
	"context"
	"fmt"
	"maps"
	"strconv"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	kyaml "sigs.k8s.io/yaml"
)

// TemplateHelmChartCRs templates the HelmChart CRs from release data using the template engine and config values
func (m *appReleaseManager) TemplateHelmChartCRs(ctx context.Context, configValues types.AppConfigValues) ([]*kotsv1beta2.HelmChart, error) {
	if m.releaseData == nil {
		return nil, fmt.Errorf("release data not initialized")
	}

	if m.templateEngine == nil {
		return nil, fmt.Errorf("template engine not initialized")
	}

	// Get HelmChart CRs from release data
	helmChartCRs := m.releaseData.HelmChartCRs
	if len(helmChartCRs) == 0 {
		return []*kotsv1beta2.HelmChart{}, nil
	}

	templatedCRs := make([]*kotsv1beta2.HelmChart, 0, len(helmChartCRs))

	for _, helmChartCR := range helmChartCRs {
		if helmChartCR == nil {
			continue
		}

		// Marshal the HelmChart CR to YAML for templating
		helmChartYAML, err := kyaml.Marshal(helmChartCR)
		if err != nil {
			return nil, fmt.Errorf("marshal helm chart CR: %w", err)
		}

		// Parse the YAML as a template
		if err := m.templateEngine.Parse(string(helmChartYAML)); err != nil {
			return nil, fmt.Errorf("parse helm chart template: %w", err)
		}

		// Execute the template with config values
		templatedYAML, err := m.templateEngine.Execute(configValues)
		if err != nil {
			return nil, fmt.Errorf("execute helm chart template: %w", err)
		}

		// Unmarshal the templated YAML back to HelmChart CR
		var templatedCR kotsv1beta2.HelmChart
		if err := kyaml.Unmarshal([]byte(templatedYAML), &templatedCR); err != nil {
			return nil, fmt.Errorf("unmarshal templated helm chart CR: %w", err)
		}

		templatedCRs = append(templatedCRs, &templatedCR)
	}

	return templatedCRs, nil
}

// GenerateHelmValues generates Helm values for a single templated HelmChart custom resource
func (m *appReleaseManager) GenerateHelmValues(ctx context.Context, templatedCR *kotsv1beta2.HelmChart) (map[string]any, error) {
	if templatedCR == nil {
		return nil, fmt.Errorf("templated CR is nil")
	}

	// Start with the base values
	mergedValues := templatedCR.Spec.Values
	if mergedValues == nil {
		mergedValues = map[string]kotsv1beta2.MappedChartValue{}
	}

	// Process OptionalValues based on their conditions
	for _, optionalValue := range templatedCR.Spec.OptionalValues {
		if optionalValue == nil {
			continue
		}

		// Check the "when" condition
		shouldInclude, err := strconv.ParseBool(optionalValue.When)
		if err != nil {
			return nil, fmt.Errorf("parse when condition on optional value: %w", err)
		}
		if !shouldInclude {
			continue
		}

		if optionalValue.RecursiveMerge {
			// Use KOTS merge function for recursive merge
			mergedValues = kotsv1beta2.MergeHelmChartValues(mergedValues, optionalValue.Values)
		} else {
			// Direct key replacement
			maps.Copy(mergedValues, optionalValue.Values)
		}
	}

	// Convert MappedChartValue to standard Go interface{} using GetHelmValues
	chartValues, err := templatedCR.Spec.GetHelmValues(mergedValues)
	if err != nil {
		return nil, fmt.Errorf("get helm values for chart %s: %w", templatedCR.Name, err)
	}

	return chartValues, nil
}
