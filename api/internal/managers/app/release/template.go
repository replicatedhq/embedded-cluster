package release

import (
	"context"
	"fmt"
	"maps"
	"strconv"

	"github.com/replicatedhq/embedded-cluster/api/pkg/template"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	troubleshootloader "github.com/replicatedhq/troubleshoot/pkg/loader"
	kyaml "sigs.k8s.io/yaml"
)

// ExtractAppPreflightSpec extracts and merges preflight specifications from app releases
func (m *appReleaseManager) ExtractAppPreflightSpec(ctx context.Context, configValues types.AppConfigValues, proxySpec *ecv1beta1.ProxySpec, registrySettings *types.RegistrySettings) (*troubleshootv1beta2.PreflightSpec, error) {
	// Template Helm chart CRs with config values
	templatedCRs, err := m.templateHelmChartCRs(configValues, proxySpec, registrySettings)
	if err != nil {
		return nil, fmt.Errorf("template helm chart CRs: %w", err)
	}

	var allPreflightSpecs []troubleshootv1beta2.PreflightSpec

	// Iterate over each templated CR and extract preflights from its Helm chart archive
	for _, cr := range templatedCRs {
		// Dry run the Helm chart archive to get manifests
		manifests, err := m.dryRunHelmChart(ctx, cr)
		if err != nil {
			return nil, fmt.Errorf("dry run helm chart %s: %w", cr.Name, err)
		}

		// Extract troubleshoot kinds from manifests
		tsKinds, err := extractTroubleshootKinds(ctx, manifests)
		if err != nil {
			return nil, fmt.Errorf("extract troubleshoot kinds from chart %s: %w", cr.Name, err)
		}

		// Extract preflight specs from troubleshoot kinds
		if tsKinds != nil && len(tsKinds.PreflightsV1Beta2) > 0 {
			for _, preflight := range tsKinds.PreflightsV1Beta2 {
				allPreflightSpecs = append(allPreflightSpecs, preflight.Spec)
			}
		}
	}

	// Merge all preflight specs into a single spec
	mergedSpec := mergePreflightSpecs(allPreflightSpecs)

	return mergedSpec, nil
}

// templateHelmChartCRs templates the HelmChart CRs from release data using the template engine and config values
func (m *appReleaseManager) templateHelmChartCRs(configValues types.AppConfigValues, proxySpec *ecv1beta1.ProxySpec, registrySettings *types.RegistrySettings) ([]*kotsv1beta2.HelmChart, error) {
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
		if len(helmChartCR) == 0 {
			continue
		}

		// Parse the YAML as a template
		if err := m.templateEngine.Parse(string(helmChartCR)); err != nil {
			return nil, fmt.Errorf("parse helm chart template: %w", err)
		}

		// Execute the template with config values
		execOptions := []template.ExecOption{
			template.WithProxySpec(proxySpec),
			template.WithRegistrySettings(registrySettings),
		}
		templatedYAML, err := m.templateEngine.Execute(configValues, execOptions...)
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

// dryRunHelmChart finds the corresponding chart archive and performs a dry run templating of a Helm chart
func (m *appReleaseManager) dryRunHelmChart(ctx context.Context, templatedCR *kotsv1beta2.HelmChart) ([][]byte, error) {
	if templatedCR == nil {
		return nil, fmt.Errorf("templated CR is nil")
	}

	// Check if the chart should be excluded
	if !templatedCR.Spec.Exclude.IsEmpty() {
		exclude, err := templatedCR.Spec.Exclude.Boolean()
		if err != nil {
			return nil, fmt.Errorf("parse templated CR exclude: %w", err)
		}
		if exclude {
			return nil, nil
		}
	}

	// Find the corresponding chart archive for this HelmChart CR
	chartArchive, err := findChartArchive(m.releaseData.HelmChartArchives, templatedCR)
	if err != nil {
		return nil, fmt.Errorf("find chart archive for %s: %w", templatedCR.Name, err)
	}

	// Generate Helm values from the templated CR
	helmValues, err := generateHelmValues(templatedCR)
	if err != nil {
		return nil, fmt.Errorf("generate helm values for %s: %w", templatedCR.Name, err)
	}

	// Create a Helm client for dry run templating
	helmClient, err := helm.NewClient(helm.HelmOptions{})
	if err != nil {
		return nil, fmt.Errorf("create helm client: %w", err)
	}
	defer helmClient.Close()

	// Write chart archive to a temporary file
	chartPath, err := writeChartArchiveToTemp(chartArchive)
	if err != nil {
		return nil, fmt.Errorf("write chart archive to temp: %w", err)
	}
	defer helpers.Remove(chartPath)

	// Fallback to admin console namespace if namespace is not set
	namespace := templatedCR.GetNamespace()
	if namespace == "" {
		namespace = constants.KotsadmNamespace
	}

	// Prepare install options for dry run
	installOpts := helm.InstallOptions{
		ReleaseName:  templatedCR.GetReleaseName(),
		ChartPath:    chartPath,
		ChartVersion: templatedCR.GetChartVersion(),
		Values:       helmValues,
		Namespace:    namespace,
	}

	// Perform dry run rendering
	manifests, err := helmClient.Render(ctx, installOpts)
	if err != nil {
		return nil, fmt.Errorf("render helm chart %s: %w", templatedCR.Name, err)
	}

	return manifests, nil
}

// generateHelmValues generates Helm values for a single templated HelmChart custom resource
func generateHelmValues(templatedCR *kotsv1beta2.HelmChart) (map[string]any, error) {
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
	helmValues, err := templatedCR.Spec.GetHelmValues(mergedValues)
	if err != nil {
		return nil, fmt.Errorf("get helm values for chart %s: %w", templatedCR.Name, err)
	}

	return helmValues, nil
}

// extractTroubleshootKinds extracts troubleshoot specifications from Helm chart manifests
func extractTroubleshootKinds(ctx context.Context, manifests [][]byte) (*troubleshootloader.TroubleshootKinds, error) {
	// Convert [][]byte manifests to []string for troubleshootloader
	rawSpecs := make([]string, len(manifests))
	for i, manifest := range manifests {
		rawSpecs[i] = string(manifest)
	}

	// Use troubleshootloader to parse all specs
	tsKinds, err := troubleshootloader.LoadSpecs(ctx, troubleshootloader.LoadOptions{
		RawSpecs: rawSpecs,
	})
	if err != nil {
		return nil, fmt.Errorf("load troubleshoot specs: %w", err)
	}

	return tsKinds, nil
}
