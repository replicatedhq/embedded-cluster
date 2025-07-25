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
  optionalValues:
  - when: '{{repl ConfigOption "enable_persistence"}}'
    values:
      persistence:
        enabled: true
        size: 10Gi
  - when: '{{repl ConfigOption "disable_monitoring"}}'
    values:
      monitoring:
        enabled: false
`),
			},
			configValues: types.AppConfigValues{
				"chart_name":         {Value: "NGINX"},
				"image_tag":          {Value: "1.20.0"},
				"app_name":           {Value: "myapp"},
				"enable_persistence": {Value: "true"},
				"disable_monitoring": {Value: "false"},
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
  optionalValues:
  - when: "true"
    values:
      persistence:
        enabled: true
        size: 10Gi
  - when: "false"
    values:
      monitoring:
        enabled: false
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
  optionalValues:
  - when: '{{repl ParseBool (ConfigOption "enable_resources") | not}}'
    values:
      resources:
        limits:
          memory: 128Mi
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
  optionalValues:
  - when: '{{repl ConfigOptionEquals "redis_persistence" "true"}}'
    recursiveMerge: true
    values:
      persistence:
        enabled: true
        size: 8Gi
`),
			},
			configValues: types.AppConfigValues{
				"chart1_name":        {Value: "NGINX"},
				"chart1_version":     {Value: "1.20.0"},
				"chart1_replicas":    {Value: "3"},
				"chart2_name":        {Value: "redis"},
				"service_type":       {Value: "clusterip"},
				"service_port":       {Value: "6379"},
				"enable_resources":   {Value: "false"},
				"redis_persistence":  {Value: "true"},
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
  optionalValues:
  - when: "true"
    values:
      resources:
        limits:
          memory: 128Mi
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
  optionalValues:
  - when: "true"
    recursiveMerge: true
    values:
      persistence:
        enabled: true
        size: 8Gi
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
			config := createTestConfig()

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

func TestAppReleaseManager_GenerateHelmValues(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		templatedCR *kotsv1beta2.HelmChart
		expected    map[string]any
		expectError bool
	}{
		{
			name:        "nil templated CR",
			templatedCR: nil,
			expected:    nil,
			expectError: true,
		},
		{
			name: "helm chart with simple values",
			templatedCR: createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: nginx-chart
  namespace: default
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    replicaCount: "3"
    image:
      repository: nginx
      tag: "1.20.0"
    service:
      type: ClusterIP
      port: 80
`),
			expected: map[string]any{
				"replicaCount": "3", // from base values
				"image": map[string]any{
					"repository": "nginx",  // from base values
					"tag":        "1.20.0", // from base values
				},
				"service": map[string]any{
					"type": "ClusterIP", // from base values
					"port": float64(80), // from base values
				},
			},
			expectError: false,
		},
		{
			name: "helm chart with optional values",
			templatedCR: createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: redis-chart
spec:
  chart:
    name: redis
    chartVersion: "2.0.0"
  values:
    persistence:
      enabled: false
  optionalValues:
  - when: "true"
    values:
      persistence:
        enabled: true
        size: "10Gi"
`),
			expected: map[string]any{
				"persistence": map[string]any{
					"enabled": true,   // from optional values (overrode base value false)
					"size":    "10Gi", // from optional values (new key)
				},
			},
			expectError: false,
		},
		{
			name: "helm chart with recursive merge",
			templatedCR: createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: merge-chart
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    service:
      type: ClusterIP
      port: 80
    resources:
      limits:
        cpu: "100m"
        memory: "64Mi"
    replicaCount: "1"
  optionalValues:
  - when: "true"
    recursiveMerge: true
    values:
      service:
        type: NodePort
        nodePort: 30080
      resources:
        limits:
          memory: "128Mi"
        requests:
          cpu: "50m"
      replicaCount: "3"
`),
			expected: map[string]any{
				"service": map[string]any{
					"type":     "NodePort",     // from optional values (overrode base value via recursive merge)
					"port":     float64(80),    // from base values (preserved)
					"nodePort": float64(30080), // from optional values (added via recursive merge)
				},
				"resources": map[string]any{
					"limits": map[string]any{
						"cpu":    "100m",  // from base values (preserved)
						"memory": "128Mi", // from optional values (overrode base value via recursive merge)
					},
					"requests": map[string]any{
						"cpu": "50m", // from optional values (added via recursive merge)
					},
				},
				"replicaCount": "3", // from optional values (overrode base value via recursive merge)
			},
			expectError: false,
		},
		{
			name: "helm chart with direct key replacement (no recursive merge)",
			templatedCR: createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: replace-chart
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    service:
      type: ClusterIP
      port: 80
      annotations:
        example: "original"
    resources:
      limits:
        cpu: "100m"
        memory: "128Mi"
  optionalValues:
  - when: "true"
    recursiveMerge: false
    values:
      service:
        type: NodePort
        nodePort: 30080
      resources:
        requests:
          cpu: "50m"
`),
			expected: map[string]any{
				"service": map[string]any{
					"type":     "NodePort",     // from optional values (direct replacement)
					"nodePort": float64(30080), // from optional values (direct replacement)
					// Note: port=80 and annotations are GONE because entire service key was replaced
				},
				"resources": map[string]any{
					"requests": map[string]any{
						"cpu": "50m", // from optional values (direct replacement)
					},
					// Note: limits.cpu and limits.memory are GONE because entire resources key was replaced
				},
			},
			expectError: false,
		},
		{
			name: "helm chart with when condition false",
			templatedCR: createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: false-when-chart
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    replicaCount: "1"
  optionalValues:
  - when: "false"
    values:
      replicaCount: "3"
      extraConfig: "should not appear"
`),
			expected: map[string]any{
				"replicaCount": "1", // from base values (optional values skipped due to when=false)
			},
			expectError: false,
		},
		{
			name: "helm chart with multiple optional values",
			templatedCR: createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: multi-optional-chart
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    replicaCount: "1"
  optionalValues:
  - when: "true"
    values:
      persistence:
        enabled: true
  - when: "false"
    values:
      debugging:
        enabled: true
  - when: "true"
    recursiveMerge: true
    values:
      persistence:
        size: "10Gi"
      monitoring:
        enabled: true
`),
			expected: map[string]any{
				"replicaCount": "1", // from base values
				"persistence": map[string]any{
					"enabled": true,   // from 1st optional values (when=true, direct replacement)
					"size":    "10Gi", // from 3rd optional values (when=true, recursive merge)
				},
				"monitoring": map[string]any{
					"enabled": true, // from 3rd optional values (when=true, recursive merge)
				},
				// Note: debugging is NOT present because 2nd optional values had when=false
			},
			expectError: false,
		},
		{
			name: "helm chart with no base values",
			templatedCR: createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: no-base-values-chart
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  optionalValues:
  - when: "true"
    values:
      replicaCount: "3"
      service:
        type: ClusterIP
`),
			expected: map[string]any{
				"replicaCount": "3", // from optional values (no base values)
				"service": map[string]any{
					"type": "ClusterIP", // from optional values (no base values)
				},
			},
			expectError: false,
		},
		{
			name: "clear example of recursive merge vs direct replacement",
			templatedCR: createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: merge-comparison-chart
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    database:
      host: "localhost"
      port: 5432
      ssl: true
      timeout: 10
    cache:
      type: "memory"
      size: "1Gi"
      ttl: 1800
  optionalValues:
  - when: "true"
    recursiveMerge: true
    values:
      database:
        host: "prod-db"
        password: "secret"
        timeout: 30
  - when: "true"
    recursiveMerge: false
    values:
      cache:
        type: "redis"
        ttl: 3600
`),
			expected: map[string]any{
				"database": map[string]any{
					"host":     "prod-db",         // from recursive merge optional values (overrode base value)
					"port":     float64(5432),     // from base values (preserved)
					"ssl":      true,              // from base values (preserved)
					"password": "secret",          // from recursive merge optional values (added)
					"timeout":  float64(30),       // from recursive merge optional values (overrode base value)
				},
				"cache": map[string]any{
					"type": "redis",         // from direct replacement optional values
					"ttl":  float64(3600),   // from direct replacement optional values
					// Note: size="1Gi" is GONE because entire cache key was replaced
				},
			},
			expectError: false,
		},
		{
			name: "helm chart with invalid when condition",
			templatedCR: createHelmChartFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: invalid-when-chart
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    replicaCount: "1"
  optionalValues:
  - when: "invalid-boolean"
    values:
      replicaCount: "3"
`),
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			manager := NewAppReleaseManager(config)

			result, err := manager.GenerateHelmValues(ctx, tt.templatedCR)

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

// Helper function to create test config for template engine
func createTestConfig() kotsv1beta1.Config {
	return kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "test_group",
					Items: []kotsv1beta1.ConfigItem{
						{Name: "chart_name", Type: "text", Value: multitype.FromString("nginx")},
						{Name: "image_tag", Type: "text", Value: multitype.FromString("1.20.0")},
						{Name: "app_name", Type: "text", Value: multitype.FromString("myapp")},
						{Name: "chart1_name", Type: "text", Value: multitype.FromString("nginx")},
						{Name: "chart1_version", Type: "text", Value: multitype.FromString("1.20.0")},
						{Name: "chart1_replicas", Type: "text", Value: multitype.FromString("3")},
						{Name: "chart2_name", Type: "text", Value: multitype.FromString("redis")},
						{Name: "service_type", Type: "text", Value: multitype.FromString("ClusterIP")},
						{Name: "service_port", Type: "text", Value: multitype.FromString("6379")},
						{Name: "enable_resources", Type: "text", Value: multitype.FromString("false")},
						{Name: "redis_persistence", Type: "text", Value: multitype.FromString("true")},
						{Name: "enable_persistence", Type: "text", Value: multitype.FromString("true")},
						{Name: "disable_monitoring", Type: "text", Value: multitype.FromString("false")},
					},
				},
			},
		},
	}
}
