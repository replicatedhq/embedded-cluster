package release

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kyaml "sigs.k8s.io/yaml"
)

func TestAppReleaseManager_TemplateHelmChartCRs(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		helmChartCRs []*kotsv1beta2.HelmChart
		configValues types.AppConfigValues
		expected     []*kotsv1beta2.HelmChart
		expectError  bool
	}{
		{
			name:         "empty helm chart CRs",
			helmChartCRs: []*kotsv1beta2.HelmChart{},
			configValues: types.AppConfigValues{},
			expected:     []*kotsv1beta2.HelmChart{},
			expectError:  false,
		},
		{
			name: "single helm chart with repl templating",
			helmChartCRs: []*kotsv1beta2.HelmChart{
				createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: test-chart
  namespace: default
spec:
  chart:
    name: '{{repl ConfigOption "chart_name" | lower}}'
    chartVersion: "1.0.0"
  values:
    image:
      tag: '{{repl ConfigOption "image_tag"}}'
    name: '{{repl ConfigOption "app_name" | upper}}'
`),
			},
			configValues: types.AppConfigValues{
				"chart_name": {Value: "NGINX"},
				"image_tag":  {Value: "1.20.0"},
				"app_name":   {Value: "myapp"},
			},
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: test-chart
  namespace: default
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    image:
      tag: "1.20.0"
    name: MYAPP
`),
			},
			expectError: false,
		},
		{
			name: "multiple helm charts with mixed templating",
			helmChartCRs: []*kotsv1beta2.HelmChart{
				createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: chart-1
  namespace: default
spec:
  chart:
    name: '{{repl ConfigOption "chart1_name" | lower}}'
    chartVersion: '{{repl ConfigOption "chart1_version"}}'
  values:
    replicas: '{{repl ConfigOption "chart1_replicas"}}'
`),
				createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: chart-2
  namespace: kube-system
spec:
  chart:
    name: '{{repl ConfigOption "chart2_name"}}'
    chartVersion: "2.0.0"
  values:
    service:
      type: '{{repl ConfigOption "service_type" | upper}}'
      port: '{{repl ConfigOption "service_port"}}'
`),
			},
			configValues: types.AppConfigValues{
				"chart1_name":     {Value: "NGINX"},
				"chart1_version":  {Value: "1.20.0"},
				"chart1_replicas": {Value: "3"},
				"chart2_name":     {Value: "redis"},
				"service_type":    {Value: "clusterip"},
				"service_port":    {Value: "6379"},
			},
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: chart-1
  namespace: default
spec:
  chart:
    name: nginx
    chartVersion: "1.20.0"
  values:
    replicas: "3"
`),
				createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: chart-2
  namespace: kube-system
spec:
  chart:
    name: redis
    chartVersion: "2.0.0"
  values:
    service:
      type: CLUSTERIP
      port: "6379"
`),
			},
			expectError: false,
		},
		{
			name: "skip nil helm chart",
			helmChartCRs: []*kotsv1beta2.HelmChart{
				nil,
				createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: valid-chart
  namespace: default
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
`),
			},
			configValues: types.AppConfigValues{},
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: valid-chart
  namespace: default
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
`),
			},
			expectError: false,
		},
		{
			name:         "nil helm chart CRs",
			helmChartCRs: nil,
			configValues: types.AppConfigValues{},
			expected:     []*kotsv1beta2.HelmChart{},
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a basic config for the template engine
			config := kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "test_group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "chart_name",
									Type:  "text",
									Value: multitype.FromString("nginx"),
								},
								{
									Name:  "image_tag",
									Type:  "text",
									Value: multitype.FromString("1.20.0"),
								},
								{
									Name:  "app_name",
									Type:  "text",
									Value: multitype.FromString("myapp"),
								},
								{
									Name:  "chart1_name",
									Type:  "text",
									Value: multitype.FromString("nginx"),
								},
								{
									Name:  "chart1_version",
									Type:  "text",
									Value: multitype.FromString("1.20.0"),
								},
								{
									Name:  "chart1_replicas",
									Type:  "text",
									Value: multitype.FromString("3"),
								},
								{
									Name:  "chart2_name",
									Type:  "text",
									Value: multitype.FromString("redis"),
								},
								{
									Name:  "service_type",
									Type:  "text",
									Value: multitype.FromString("ClusterIP"),
								},
								{
									Name:  "service_port",
									Type:  "text",
									Value: multitype.FromString("6379"),
								},
							},
						},
					},
				},
			}

			// Create release data
			releaseData := &release.ReleaseData{
				HelmChartCRs: tt.helmChartCRs,
			}

			// Create manager
			manager := NewAppReleaseManager(
				config,
				WithReleaseData(releaseData),
			)

			// Execute the function
			result, err := manager.TemplateHelmChartCRs(ctx, tt.configValues)

			// Check error expectation
			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function to create HelmChart from YAML string
func createHelmChartFromYAML(yamlStr string) *kotsv1beta2.HelmChart {
	var chart kotsv1beta2.HelmChart
	err := kyaml.Unmarshal([]byte(yamlStr), &chart)
	if err != nil {
		panic(err)
	}
	return &chart
}
