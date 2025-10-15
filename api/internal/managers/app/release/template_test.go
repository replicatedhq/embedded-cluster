package release

import (
	"context"
	"fmt"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	"github.com/replicatedhq/kotskinds/multitype"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	troubleshootloader "github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kyaml "sigs.k8s.io/yaml"
)

func TestAppReleaseManager_ExtractAppPreflightSpec(t *testing.T) {

	tests := []struct {
		name          string
		helmChartCRs  [][]byte
		chartArchives [][]byte
		configValues  types.AppConfigValues
		proxySpec     *ecv1beta1.ProxySpec
		expectedSpec  *troubleshootv1beta2.PreflightSpec
		expectError   bool
		errorContains string
	}{
		{
			name:         "no helm charts returns nil",
			helmChartCRs: [][]byte{},
			configValues: types.AppConfigValues{},
			proxySpec:    &ecv1beta1.ProxySpec{},
			expectedSpec: nil,
			expectError:  false,
		},
		{
			name: "single chart with preflight spec and templating",
			helmChartCRs: [][]byte{
				[]byte(`apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: myapp-chart
spec:
  chart:
    name: test-chart
    chartVersion: "1.0.0"
  values:
    checkName: '{{repl ConfigOption "check_name"}}'`),
			},
			chartArchives: [][]byte{
				createTarGzArchive(t, map[string]string{
					"test-chart/Chart.yaml": `apiVersion: v2
name: test-chart
version: 1.0.0
description: A test Helm chart with preflights`,
					"test-chart/templates/preflight.yaml": `apiVersion: troubleshoot.sh/v1beta2
kind: Preflight
metadata:
  name: preflight-check
spec:
  analyzers:
  - clusterVersion:
      checkName: "{{ .Values.checkName }}"
      outcomes:
      - pass:
          when: ">= 1.16.0"
          message: "Kubernetes version is supported"
      - fail:
          message: "Kubernetes version is not supported"
  collectors:
  - clusterInfo: {}`,
				}),
			},
			configValues: types.AppConfigValues{
				"check_name": {Value: "K8s Version Validation"},
			},
			proxySpec: &ecv1beta1.ProxySpec{},
			expectedSpec: &troubleshootv1beta2.PreflightSpec{
				Analyzers: []*troubleshootv1beta2.Analyze{
					{
						ClusterVersion: &troubleshootv1beta2.ClusterVersion{
							AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
								CheckName: "K8s Version Validation",
							},
							Outcomes: []*troubleshootv1beta2.Outcome{
								{
									Pass: &troubleshootv1beta2.SingleOutcome{
										When:    ">= 1.16.0",
										Message: "Kubernetes version is supported",
									},
								},
								{
									Fail: &troubleshootv1beta2.SingleOutcome{
										Message: "Kubernetes version is not supported",
									},
								},
							},
						},
					},
				},
				Collectors: []*troubleshootv1beta2.Collect{
					{
						ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
					},
				},
			},
			expectError: false,
		},
		{
			name: "multiple charts with merged preflight specs and templating",
			helmChartCRs: [][]byte{
				[]byte(`apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: frontend-chart
spec:
  chart:
    name: chart1
    chartVersion: "1.0.0"
  values:
    versionCheckName: '{{repl ConfigOption "version_check_name"}}'`),
				[]byte(`apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: backend-chart
spec:
  chart:
    name: chart2
    chartVersion: "1.0.0"
  values:
    resourceCheckName: '{{repl ConfigOption "resource_check_name"}}'`),
			},
			chartArchives: [][]byte{
				createTarGzArchive(t, map[string]string{
					"chart1/Chart.yaml": `apiVersion: v2
name: chart1
version: 1.0.0`,
					"chart1/templates/preflight.yaml": `apiVersion: troubleshoot.sh/v1beta2
kind: Preflight
metadata:
  name: preflight-check
spec:
  analyzers:
  - clusterVersion:
      checkName: "{{ .Values.versionCheckName }}"
  collectors:
  - clusterInfo: {}`,
				}),
				createTarGzArchive(t, map[string]string{
					"chart2/Chart.yaml": `apiVersion: v2
name: chart2
version: 1.0.0`,
					"chart2/templates/preflight.yaml": `apiVersion: troubleshoot.sh/v1beta2
kind: Preflight
metadata:
  name: node-resources-check
spec:
  analyzers:
  - nodeResources:
      checkName: "{{ .Values.resourceCheckName }}"
      outcomes:
      - fail:
          when: "count() < 3"
          message: "At least 3 nodes are required"
      - pass:
          message: "Sufficient nodes available"
  collectors:
  - clusterResources: {}`,
				}),
			},
			configValues: types.AppConfigValues{
				"version_check_name":  {Value: "Custom K8s Version Check"},
				"resource_check_name": {Value: "Custom Node Resource Check"},
			},
			proxySpec: &ecv1beta1.ProxySpec{},
			expectedSpec: &troubleshootv1beta2.PreflightSpec{
				Analyzers: []*troubleshootv1beta2.Analyze{
					{
						ClusterVersion: &troubleshootv1beta2.ClusterVersion{
							AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
								CheckName: "Custom K8s Version Check",
							},
						},
					},
					{
						NodeResources: &troubleshootv1beta2.NodeResources{
							AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
								CheckName: "Custom Node Resource Check",
							},
							Outcomes: []*troubleshootv1beta2.Outcome{
								{
									Fail: &troubleshootv1beta2.SingleOutcome{
										When:    "count() < 3",
										Message: "At least 3 nodes are required",
									},
								},
								{
									Pass: &troubleshootv1beta2.SingleOutcome{
										Message: "Sufficient nodes available",
									},
								},
							},
						},
					},
				},
				Collectors: []*troubleshootv1beta2.Collect{
					{
						ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
					},
					{
						ClusterResources: &troubleshootv1beta2.ClusterResources{},
					},
				},
			},
			expectError: false,
		},
		{
			name: "chart with no preflights returns empty spec",
			helmChartCRs: [][]byte{
				[]byte(`apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: simple-chart
spec:
  chart:
    name: simple-chart
    chartVersion: "1.0.0"`),
			},
			chartArchives: [][]byte{
				createTestChartArchive(t, "simple-chart", "1.0.0"),
			},
			configValues: types.AppConfigValues{},
			proxySpec:    &ecv1beta1.ProxySpec{},
			expectedSpec: nil,
			expectError:  false,
		},
		{
			name: "chart with proxy template functions",
			helmChartCRs: [][]byte{
				[]byte(`apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: proxy-chart
spec:
  chart:
    name: test-chart
    chartVersion: "1.0.0"
  values:
    httpProxy: '{{repl HTTPProxy}}'
    httpsProxy: '{{repl HTTPSProxy}}'
    noProxy: '{{repl NoProxy}}'`),
			},
			chartArchives: [][]byte{
				createTarGzArchive(t, map[string]string{
					"test-chart/Chart.yaml": `apiVersion: v2
name: test-chart
version: 1.0.0
description: A test Helm chart with proxy settings`,
					"test-chart/templates/preflight.yaml": `apiVersion: troubleshoot.sh/v1beta2
kind: Preflight
metadata:
  name: proxy-preflight
spec:
  analyzers:
  - http:
      checkName: "HTTP Proxy Check"
      outcomes:
      - pass:
          when: "statusCode == 200"
          message: "HTTP proxy is accessible"
      - fail:
          message: "HTTP proxy is not accessible"
  collectors:
  - http:
      name: proxy-test
      get:
        url: "{{ .Values.httpsProxy }}/healthz"`,
				}),
			},
			configValues: types.AppConfigValues{},
			proxySpec: &ecv1beta1.ProxySpec{
				HTTPProxy:  "http://proxy.example.com:8080",
				HTTPSProxy: "https://proxy.example.com:8443",
				NoProxy:    "localhost,127.0.0.1,.cluster.local",
			},
			expectedSpec: &troubleshootv1beta2.PreflightSpec{
				Analyzers: []*troubleshootv1beta2.Analyze{
					{
						HTTP: &troubleshootv1beta2.HTTPAnalyze{
							AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
								CheckName: "HTTP Proxy Check",
							},
							Outcomes: []*troubleshootv1beta2.Outcome{
								{
									Pass: &troubleshootv1beta2.SingleOutcome{
										When:    "statusCode == 200",
										Message: "HTTP proxy is accessible",
									},
								},
								{
									Fail: &troubleshootv1beta2.SingleOutcome{
										Message: "HTTP proxy is not accessible",
									},
								},
							},
						},
					},
				},
				Collectors: []*troubleshootv1beta2.Collect{
					{
						HTTP: &troubleshootv1beta2.HTTP{
							Name: "proxy-test",
							Get: &troubleshootv1beta2.Get{
								URL: "https://proxy.example.com:8443/healthz",
							},
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a basic config for the template engine
			config := createTestConfig()

			// Create release data
			releaseData := &release.ReleaseData{
				HelmChartCRs:      tt.helmChartCRs,
				HelmChartArchives: tt.chartArchives,
			}

			// Create real helm client
			hcli, err := helm.NewClient(helm.HelmOptions{
				HelmPath:   "helm", // use the helm binary in PATH
				K8sVersion: "v1.33.0",
			})
			require.NoError(t, err)

			// Create manager
			manager, err := NewAppReleaseManager(
				config,
				WithReleaseData(releaseData),
				WithHelmClient(hcli),
			)
			require.NoError(t, err)

			// Execute the function
			result, err := manager.ExtractAppPreflightSpec(t.Context(), tt.configValues, tt.proxySpec, nil)

			// Check error expectation
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedSpec, result)
		})
	}
}

func TestAppReleaseManager_templateHelmChartCRs(t *testing.T) {
	tests := []struct {
		name             string
		helmChartCRs     [][]byte
		configValues     types.AppConfigValues
		proxySpec        *ecv1beta1.ProxySpec
		registrySettings *types.RegistrySettings
		isAirgap         bool
		expected         []*kotsv1beta2.HelmChart
		expectError      bool
	}{
		{
			name:             "empty helm chart CRs",
			helmChartCRs:     [][]byte{},
			configValues:     types.AppConfigValues{},
			proxySpec:        &ecv1beta1.ProxySpec{},
			registrySettings: nil,
			expected:         []*kotsv1beta2.HelmChart{},
			expectError:      false,
		},
		{
			name: "single helm chart with repl templating",
			helmChartCRs: [][]byte{
				[]byte(`
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
			proxySpec: &ecv1beta1.ProxySpec{},
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartCRFromYAML(`
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
			helmChartCRs: [][]byte{
				[]byte(`
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
				[]byte(`
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
				"chart1_name":       {Value: "NGINX"},
				"chart1_version":    {Value: "1.20.0"},
				"chart1_replicas":   {Value: "3"},
				"chart2_name":       {Value: "redis"},
				"service_type":      {Value: "clusterip"},
				"service_port":      {Value: "6379"},
				"enable_resources":  {Value: "false"},
				"redis_persistence": {Value: "true"},
			},
			proxySpec: &ecv1beta1.ProxySpec{},
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartCRFromYAML(`
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
				createHelmChartCRFromYAML(`
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
			helmChartCRs: [][]byte{
				nil,
				[]byte(`
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
			proxySpec:    &ecv1beta1.ProxySpec{},
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartCRFromYAML(`
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
			proxySpec:    &ecv1beta1.ProxySpec{},
			expected:     []*kotsv1beta2.HelmChart{},
			expectError:  false,
		},
		{
			name: "helm chart with proxy template functions",
			helmChartCRs: [][]byte{
				[]byte(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: proxy-enabled-chart
  namespace: default
spec:
  chart:
    name: nginx-proxy
    chartVersion: "1.0.0"
  values:
    proxy:
      http: '{{repl HTTPProxy}}'
      https: '{{repl HTTPSProxy}}'
      noProxy: '{{repl NoProxy | join ","}}'
    env:
      HTTP_PROXY: '{{repl HTTPProxy}}'
      HTTPS_PROXY: '{{repl HTTPSProxy}}'
      NO_PROXY: '{{repl NoProxy | join ","}}'
  optionalValues:
  - when: '{{repl if HTTPProxy}}true{{repl else}}false{{repl end}}'
    values:
      proxyEnabled: true
`),
			},
			configValues: types.AppConfigValues{},
			proxySpec: &ecv1beta1.ProxySpec{
				HTTPProxy:  "http://corporate-proxy.example.com:8080",
				HTTPSProxy: "https://corporate-proxy.example.com:8443",
				NoProxy:    "localhost,127.0.0.1,.internal,10.0.0.0/8",
			},
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartCRFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: proxy-enabled-chart
  namespace: default
spec:
  chart:
    name: nginx-proxy
    chartVersion: "1.0.0"
  values:
    proxy:
      http: "http://corporate-proxy.example.com:8080"
      https: "https://corporate-proxy.example.com:8443"
      noProxy: "localhost,127.0.0.1,.internal,10.0.0.0/8"
    env:
      HTTP_PROXY: "http://corporate-proxy.example.com:8080"
      HTTPS_PROXY: "https://corporate-proxy.example.com:8443"
      NO_PROXY: "localhost,127.0.0.1,.internal,10.0.0.0/8"
  optionalValues:
  - when: "true"
    values:
      proxyEnabled: true
`),
			},
			expectError: false,
		},
		{
			name: "helm chart with empty proxy spec",
			helmChartCRs: [][]byte{
				[]byte(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: no-proxy-chart
  namespace: default
spec:
  chart:
    name: nginx-no-proxy
    chartVersion: "1.0.0"
  values:
    proxy:
      http: '{{repl HTTPProxy}}'
      https: '{{repl HTTPSProxy}}'
      noProxy: '{{repl NoProxy | join ","}}'
  optionalValues:
  - when: '{{repl if HTTPProxy}}true{{repl else}}false{{repl end}}'
    values:
      proxyEnabled: true
  - when: '{{repl if not HTTPProxy}}true{{repl else}}false{{repl end}}'
    values:
      proxyEnabled: false
`),
			},
			configValues: types.AppConfigValues{},
			proxySpec:    &ecv1beta1.ProxySpec{}, // Empty proxy spec
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartCRFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: no-proxy-chart
  namespace: default
spec:
  chart:
    name: nginx-no-proxy
    chartVersion: "1.0.0"
  values:
    proxy:
      http: ""
      https: ""
      noProxy: ""
  optionalValues:
  - when: "false"
    values:
      proxyEnabled: true
  - when: "true"
    values:
      proxyEnabled: false
`),
			},
			expectError: false,
		},
		{
			name: "helm chart with registry template functions - airgap mode",
			helmChartCRs: [][]byte{
				[]byte(`
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
    image:
      repository: '{{repl HasLocalRegistry | ternary LocalRegistryHost "proxy.replicated.com"}}/{{repl HasLocalRegistry | ternary LocalRegistryNamespace "external/path"}}/nginx'
    imagePullSecrets:
      - name: '{{repl ImagePullSecretName}}'
    registry:
      host: '{{repl LocalRegistryHost}}'
      address: '{{repl LocalRegistryAddress}}'
      namespace: '{{repl LocalRegistryNamespace}}'
      secret: '{{repl LocalRegistryImagePullSecret}}'
`),
			},
			configValues: types.AppConfigValues{},
			proxySpec:    &ecv1beta1.ProxySpec{},
			registrySettings: &types.RegistrySettings{
				HasLocalRegistry:     true,
				Host:                 "10.128.0.11:5000",
				Address:              "10.128.0.11:5000/myapp",
				Namespace:            "myapp",
				ImagePullSecretName:  "embedded-cluster-registry",
				ImagePullSecretValue: "dGVzdC1zZWNyZXQtdmFsdWU=",
			},
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartCRFromYAML(`
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
    image:
      repository: "10.128.0.11:5000/myapp/nginx"
    imagePullSecrets:
      - name: "embedded-cluster-registry"
    registry:
      host: "10.128.0.11:5000"
      address: "10.128.0.11:5000/myapp"
      namespace: "myapp"
      secret: "dGVzdC1zZWNyZXQtdmFsdWU="
`),
			},
			expectError: false,
		},
		{
			name: "helm chart with registry template functions - online mode",
			helmChartCRs: [][]byte{
				[]byte(`
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
    image:
      repository: '{{repl HasLocalRegistry | ternary LocalRegistryHost "proxy.replicated.com"}}/{{repl HasLocalRegistry | ternary LocalRegistryNamespace "external/path"}}/nginx'
    imagePullSecrets:
      - name: '{{repl ImagePullSecretName}}'
`),
			},
			configValues: types.AppConfigValues{},
			proxySpec:    &ecv1beta1.ProxySpec{},
			registrySettings: &types.RegistrySettings{
				HasLocalRegistry: false,
			},
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartCRFromYAML(`
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
    image:
      repository: "proxy.replicated.com/external/path/nginx"
    imagePullSecrets:
      - name: ""
`),
			},
			expectError: false,
		},
		{
			name: "helm chart with registry template functions - nil registry settings",
			helmChartCRs: [][]byte{
				[]byte(`
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
    image:
      repository: '{{repl HasLocalRegistry | ternary LocalRegistryHost "proxy.replicated.com"}}/{{repl HasLocalRegistry | ternary LocalRegistryNamespace "external/path"}}/nginx'
    imagePullSecrets:
      - name: '{{repl ImagePullSecretName}}'
`),
			},
			configValues:     types.AppConfigValues{},
			proxySpec:        &ecv1beta1.ProxySpec{},
			registrySettings: nil, // No registry settings provided
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartCRFromYAML(`
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
    image:
      repository: "proxy.replicated.com/external/path/nginx"
    imagePullSecrets:
      - name: ""
`),
			},
			expectError: false,
		},
		{
			name:     "helm chart with IsAirgap template function - airgap mode",
			isAirgap: true,
			helmChartCRs: [][]byte{
				[]byte(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: airgap-chart
  namespace: default
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    installationMode: '{{repl IsAirgap | ternary "airgap" "online"}}'
    downloadImages: '{{repl IsAirgap}}'
    environment:
      type: '{{repl if IsAirgap}}Airgap Installation{{repl else}}Online Installation{{repl end}}'
  optionalValues:
  - when: '{{repl IsAirgap}}'
    values:
      registry:
        enabled: true
        host: "local-registry.example.com"
  - when: '{{repl if not IsAirgap}}true{{repl else}}false{{repl end}}'
    values:
      externalAccess:
        enabled: true
`),
			},
			configValues: types.AppConfigValues{},
			proxySpec:    &ecv1beta1.ProxySpec{},
			registrySettings: &types.RegistrySettings{
				HasLocalRegistry: true,
			},
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartCRFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: airgap-chart
  namespace: default
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    installationMode: airgap
    downloadImages: "true"
    environment:
      type: Airgap Installation
  optionalValues:
  - when: "true"
    values:
      registry:
        enabled: true
        host: "local-registry.example.com"
  - when: "false"
    values:
      externalAccess:
        enabled: true
`),
			},
			expectError: false,
		},
		{
			name:     "helm chart with IsAirgap template function - online mode",
			isAirgap: false,
			helmChartCRs: [][]byte{
				[]byte(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: online-chart
  namespace: default
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    installationMode: '{{repl IsAirgap | ternary "airgap" "online"}}'
    downloadImages: '{{repl IsAirgap}}'
    environment:
      type: '{{repl if IsAirgap}}Airgap Installation{{repl else}}Online Installation{{repl end}}'
  optionalValues:
  - when: '{{repl IsAirgap}}'
    values:
      registry:
        enabled: true
  - when: '{{repl if not IsAirgap}}true{{repl else}}false{{repl end}}'
    values:
      externalAccess:
        enabled: true
        url: "https://external-api.example.com"
`),
			},
			configValues:     types.AppConfigValues{},
			proxySpec:        &ecv1beta1.ProxySpec{},
			registrySettings: nil, // No registry settings means online mode
			expected: []*kotsv1beta2.HelmChart{
				createHelmChartCRFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: online-chart
  namespace: default
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    installationMode: online
    downloadImages: "false"
    environment:
      type: Online Installation
  optionalValues:
  - when: "false"
    values:
      registry:
        enabled: true
  - when: "true"
    values:
      externalAccess:
        enabled: true
        url: "https://external-api.example.com"
`),
			},
			expectError: false,
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

			// Create real helm client
			hcli, err := helm.NewClient(helm.HelmOptions{
				HelmPath:   "helm", // use the helm binary in PATH
				K8sVersion: "v1.33.0",
			})
			require.NoError(t, err)

			// Create manager
			manager, err := NewAppReleaseManager(
				config,
				WithReleaseData(releaseData),
				WithIsAirgap(tt.isAirgap),
				WithHelmClient(hcli),
			)
			require.NoError(t, err)

			// Execute the function
			result, err := manager.(*appReleaseManager).templateHelmChartCRs(tt.configValues, tt.proxySpec, tt.registrySettings)

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

func TestAppReleaseManager_dryRunHelmChart(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		templatedCR       *kotsv1beta2.HelmChart
		helmChartArchives [][]byte
		expectError       bool
		errorContains     string
		validateManifest  func(t *testing.T, manifests [][]byte)
	}{
		{
			name:          "nil templated CR",
			templatedCR:   nil,
			expectError:   true,
			errorContains: "templated CR is nil",
		},
		{
			name: "no chart archives",
			templatedCR: createHelmChartCRFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: nginx-chart
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
`),
			helmChartArchives: [][]byte{},
			expectError:       true,
			errorContains:     "no helm chart archives found",
		},
		{
			name: "chart archive not found",
			templatedCR: createHelmChartCRFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: nginx-chart
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
`),
			helmChartArchives: [][]byte{
				createComplexChartArchive(t, "redis", "1.0.0"),
			},
			expectError:   true,
			errorContains: "no chart archive found for chart name nginx and version 1.0.0",
		},
		{
			name: "successful dry run with kotsadm namespace fallback",
			templatedCR: createHelmChartCRFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: nginx-chart
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    replicaCount: "3"
    image:
      tag: "1.20.0"
`),
			helmChartArchives: [][]byte{
				createComplexChartArchive(t, "nginx", "1.0.0"),
			},
			expectError: false,
			validateManifest: func(t *testing.T, manifests [][]byte) {
				// Should have multiple manifest files
				assert.GreaterOrEqual(t, len(manifests), 3, "should have at least 3 manifest files")

				// Convert to combined string for easier testing
				combined := ""
				for _, manifest := range manifests {
					combined += string(manifest) + "\n"
				}

				// Check that we have multiple resources
				assert.Contains(t, combined, "kind: CustomResourceDefinition")
				assert.Contains(t, combined, "kind: Deployment")
				assert.Contains(t, combined, "kind: Service")
				// Check that values were templated correctly
				assert.Contains(t, combined, "replicas: 3")
				assert.Contains(t, combined, "image: nginx:1.20.0")
				// Check kotsadm namespace fallback
				assert.Contains(t, combined, "namespace: kotsadm")
			},
		},
		{
			name: "successful dry run with custom namespace",
			templatedCR: createHelmChartCRFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: my-nginx
  namespace: kotsadm
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  releaseName: my-release
  namespace: custom-ns
  values:
    service:
      type: LoadBalancer
`),
			helmChartArchives: [][]byte{
				createComplexChartArchive(t, "nginx", "1.0.0"),
			},
			expectError: false,
			validateManifest: func(t *testing.T, manifests [][]byte) {
				// Should have multiple manifest files
				assert.GreaterOrEqual(t, len(manifests), 3, "should have at least 3 manifest files")

				// Convert to combined string for easier testing
				combined := ""
				for _, manifest := range manifests {
					combined += string(manifest) + "\n"
				}

				// Check that we have multiple resources
				assert.Contains(t, combined, "kind: CustomResourceDefinition")
				assert.Contains(t, combined, "kind: Deployment")
				assert.Contains(t, combined, "kind: Service")
				// Check that values were templated correctly
				assert.Contains(t, combined, "replicas: 1")
				assert.Contains(t, combined, "image: nginx:latest")
				// Check that custom namespace is used
				assert.Contains(t, combined, "namespace: custom-ns")
				// Check that custom release name is used
				assert.Contains(t, combined, "my-release")
				// Check that service type was templated
				assert.Contains(t, combined, "type: LoadBalancer")
			},
		},
		{
			name: "chart with exclude=false (should be processed)",
			templatedCR: createHelmChartCRFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: test-chart
  namespace: kotsadm
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  exclude: false
  values:
    replicaCount: "2"
`),
			helmChartArchives: [][]byte{
				createComplexChartArchive(t, "nginx", "1.0.0"),
			},
			expectError: false,
			validateManifest: func(t *testing.T, manifests [][]byte) {
				// Should have multiple manifest files since exclude=false
				assert.GreaterOrEqual(t, len(manifests), 3, "should have at least 3 manifest files")

				// Convert to combined string for easier testing
				combined := ""
				for _, manifest := range manifests {
					combined += string(manifest) + "\n"
				}

				// Check that resources are present
				assert.Contains(t, combined, "kind: Deployment")
				assert.Contains(t, combined, "replicas: 2")
			},
		},
		{
			name: "chart with exclude=true (should be skipped)",
			templatedCR: createHelmChartCRFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: excluded-chart
  namespace: kotsadm
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  exclude: true
`),
			helmChartArchives: [][]byte{
				createComplexChartArchive(t, "nginx", "1.0.0"),
			},
			expectError: false,
			validateManifest: func(t *testing.T, manifests [][]byte) {
				// Should be nil since chart is excluded
				assert.Nil(t, manifests, "excluded charts should return nil manifests")
			},
		},
		{
			name: "chart with mixed true/false optional values",
			templatedCR: createHelmChartCRFromYAML(`
apiVersion: kots.io/v1beta2
kind: HelmChart
metadata:
  name: mixed-optional-chart
  namespace: kotsadm
spec:
  chart:
    name: nginx
    chartVersion: "1.0.0"
  values:
    replicaCount: "2"
  optionalValues:
  - when: "false"
    values:
      replicaCount: "3"
  - when: "true"
    values:
      replicaCount: "4"
`),
			helmChartArchives: [][]byte{
				createComplexChartArchive(t, "nginx", "1.0.0"),
			},
			expectError: false,
			validateManifest: func(t *testing.T, manifests [][]byte) {
				// Should have multiple manifest files
				assert.GreaterOrEqual(t, len(manifests), 3, "should have at least 3 manifest files")

				// Convert to combined string for easier testing
				combined := ""
				for _, manifest := range manifests {
					combined += string(manifest) + "\n"
				}

				// Check that we have multiple resources
				assert.Contains(t, combined, "kind: Deployment")

				// Should be 4 from when=true, not 3 from when=false, not 1 from base values, not 2 from chart values
				assert.Contains(t, combined, "replicas: 4")
				assert.NotContains(t, combined, "replicas: 1")
				assert.NotContains(t, combined, "replicas: 2")
				assert.NotContains(t, combined, "replicas: 3")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a basic config for the template engine
			config := createTestConfig()

			// Create release data with chart archives
			releaseData := &release.ReleaseData{
				HelmChartArchives: tt.helmChartArchives,
			}

			// Create real helm client
			hcli, err := helm.NewClient(helm.HelmOptions{
				HelmPath:   "helm", // use the helm binary in PATH
				K8sVersion: "v1.33.0",
			})
			require.NoError(t, err)

			manager, err := NewAppReleaseManager(
				config,
				WithReleaseData(releaseData),
				WithHelmClient(hcli),
			)
			require.NoError(t, err)

			// Execute the function
			result, err := manager.(*appReleaseManager).dryRunHelmChart(ctx, tt.templatedCR)

			// Check error expectation
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
				return
			}

			require.NoError(t, err)

			// Validate the actual manifest content if provided
			if tt.validateManifest != nil {
				tt.validateManifest(t, result)
			}
		})
	}
}

func TestAppReleaseManager_generateHelmValues(t *testing.T) {
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
			templatedCR: createHelmChartCRFromYAML(`
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
			templatedCR: createHelmChartCRFromYAML(`
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
			templatedCR: createHelmChartCRFromYAML(`
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
			templatedCR: createHelmChartCRFromYAML(`
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
			templatedCR: createHelmChartCRFromYAML(`
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
			templatedCR: createHelmChartCRFromYAML(`
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
			templatedCR: createHelmChartCRFromYAML(`
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
			templatedCR: createHelmChartCRFromYAML(`
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
					"host":     "prod-db",     // from recursive merge optional values (overrode base value)
					"port":     float64(5432), // from base values (preserved)
					"ssl":      true,          // from base values (preserved)
					"password": "secret",      // from recursive merge optional values (added)
					"timeout":  float64(30),   // from recursive merge optional values (overrode base value)
				},
				"cache": map[string]any{
					"type": "redis",       // from direct replacement optional values
					"ttl":  float64(3600), // from direct replacement optional values
					// Note: size="1Gi" is GONE because entire cache key was replaced
				},
			},
			expectError: false,
		},
		{
			name: "helm chart with invalid when condition",
			templatedCR: createHelmChartCRFromYAML(`
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
			result, err := generateHelmValues(tt.templatedCR)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppReleaseManager_extractTroubleshootKinds(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                 string
		manifests            [][]byte
		expectError          bool
		errorContains        string
		validateTroubleshoot func(t *testing.T, tsKinds any)
	}{
		{
			name:        "empty manifests",
			manifests:   [][]byte{},
			expectError: false,
			validateTroubleshoot: func(t *testing.T, tsKinds any) {
				kinds := tsKinds.(*troubleshootloader.TroubleshootKinds)
				assert.Empty(t, kinds.PreflightsV1Beta2)
				assert.Empty(t, kinds.SupportBundlesV1Beta2)
			},
		},
		{
			name: "manifests with no troubleshoot specs",
			manifests: [][]byte{
				[]byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value`),
				[]byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 1`),
			},
			expectError: false,
			validateTroubleshoot: func(t *testing.T, tsKinds any) {
				kinds := tsKinds.(*troubleshootloader.TroubleshootKinds)
				assert.Empty(t, kinds.PreflightsV1Beta2)
				assert.Empty(t, kinds.SupportBundlesV1Beta2)
			},
		},
		{
			name: "manifest with direct Preflight resource",
			manifests: [][]byte{
				[]byte(`apiVersion: troubleshoot.sh/v1beta2
kind: Preflight
metadata:
  name: test-preflight
spec:
  collectors:
    - clusterInfo: {}
  analyzers:
    - clusterVersion:
        checkName: "Kubernetes Version"
        outcomes:
          - fail:
              when: "< 1.19.0"
              message: "Kubernetes version must be at least 1.19.0"
          - pass:
              message: "Kubernetes version is supported"`),
			},
			expectError: false,
			validateTroubleshoot: func(t *testing.T, tsKinds any) {
				kinds := tsKinds.(*troubleshootloader.TroubleshootKinds)

				// Should have 1 preflight
				assert.Len(t, kinds.PreflightsV1Beta2, 1)
				assert.Equal(t, "test-preflight", kinds.PreflightsV1Beta2[0].Name)

				// Should have no support bundles
				assert.Empty(t, kinds.SupportBundlesV1Beta2)
			},
		},
		{
			name: "manifest with Secret containing preflight.yaml",
			manifests: [][]byte{
				[]byte(`apiVersion: v1
kind: Secret
metadata:
  name: test-preflight-secret
type: Opaque
stringData:
  preflight.yaml: |
    apiVersion: troubleshoot.sh/v1beta2
    kind: Preflight
    metadata:
      name: secret-preflight
    spec:
      collectors:
        - clusterResources: {}
      analyzers:
        - nodeResources:
            checkName: "Node Resources"
            outcomes:
              - fail:
                  when: "count() < 3"
                  message: "At least 3 nodes required"
              - pass:
                  message: "Sufficient nodes available"`),
			},
			expectError: false,
			validateTroubleshoot: func(t *testing.T, tsKinds any) {
				kinds := tsKinds.(*troubleshootloader.TroubleshootKinds)

				// Should have 1 preflight
				assert.Len(t, kinds.PreflightsV1Beta2, 1)
				assert.Equal(t, "secret-preflight", kinds.PreflightsV1Beta2[0].Name)

				// Should have no support bundles
				assert.Empty(t, kinds.SupportBundlesV1Beta2)
			},
		},
		{
			name: "mixed manifests with multiple troubleshoot specs",
			manifests: [][]byte{
				[]byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: regular-config
data:
  key: value`),
				[]byte(`apiVersion: troubleshoot.sh/v1beta2
kind: Preflight
metadata:
  name: direct-preflight
spec:
  collectors:
    - clusterInfo: {}
  analyzers:
    - clusterVersion:
        checkName: "Kubernetes Version Check"
        outcomes:
          - pass:
              message: "Version is supported"`),
				[]byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: direct-supportbundle
spec:
  collectors:
    - nodeResources: {}
  analyzers:
    - nodeResources:
        checkName: "Node Resources Check"
        outcomes:
          - pass:
              message: "Resources are sufficient"`),
				[]byte(`apiVersion: v1
kind: Secret
metadata:
  name: preflight-secret
type: Opaque
stringData:
  preflight.yaml: |
    apiVersion: troubleshoot.sh/v1beta2
    kind: Preflight
    metadata:
      name: secret-preflight
    spec:
      collectors:
        - nodeResources: {}
      analyzers:
        - nodeResources:
            checkName: "Node Resources Check"
            outcomes:
              - pass:
                  message: "Resources are sufficient"`),
				[]byte(`apiVersion: v1
kind: Secret
metadata:
  name: supportbundle-secret
type: Opaque
stringData:
  support-bundle-spec: |
    apiVersion: troubleshoot.sh/v1beta2
    kind: SupportBundle
    metadata:
      name: secret-supportbundle
    spec:
      collectors:
        - clusterResources: {}
      analyzers:
        - clusterVersion:
            checkName: "Secret Support Bundle Analysis"
            outcomes:
              - pass:
                  message: "Cluster version analysis complete"`),
				[]byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: preflight-configmap
data:
  preflight.yaml: |
    apiVersion: troubleshoot.sh/v1beta2
    kind: Preflight
    metadata:
      name: configmap-preflight
    spec:
      collectors:
        - diskUsage:
            collectorName: disk-usage
            path: /tmp
      analyzers:
        - diskUsage:
            checkName: "ConfigMap Disk Usage Check"
            collectorName: disk-usage
            outcomes:
              - pass:
                  message: "Disk usage is acceptable"`),
				[]byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: supportbundle-configmap
data:
  support-bundle-spec: |
    apiVersion: troubleshoot.sh/v1beta2
    kind: SupportBundle
    metadata:
      name: configmap-supportbundle
    spec:
      collectors:
        - logs:
            selector:
              - app=test
            namespace: default
      analyzers:
        - textAnalyze:
            checkName: "ConfigMap Log Analysis"
            fileName: logs/default/test-*/test-*.log
            regex: 'warning'
            outcomes:
              - warn:
                  when: "true"
                  message: "Found warnings in logs"`),
			},
			expectError: false,
			validateTroubleshoot: func(t *testing.T, tsKinds any) {
				kinds := tsKinds.(*troubleshootloader.TroubleshootKinds)

				// Should have 3 preflights
				assert.Len(t, kinds.PreflightsV1Beta2, 3)
				preflightNames := []string{kinds.PreflightsV1Beta2[0].Name, kinds.PreflightsV1Beta2[1].Name, kinds.PreflightsV1Beta2[2].Name}
				assert.Contains(t, preflightNames, "direct-preflight")
				assert.Contains(t, preflightNames, "secret-preflight")
				assert.Contains(t, preflightNames, "configmap-preflight")

				// Should have 3 support bundles
				assert.Len(t, kinds.SupportBundlesV1Beta2, 3)
				supportBundleNames := []string{kinds.SupportBundlesV1Beta2[0].Name, kinds.SupportBundlesV1Beta2[1].Name, kinds.SupportBundlesV1Beta2[2].Name}
				assert.Contains(t, supportBundleNames, "direct-supportbundle")
				assert.Contains(t, supportBundleNames, "secret-supportbundle")
				assert.Contains(t, supportBundleNames, "configmap-supportbundle")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function
			tsKinds, err := extractTroubleshootKinds(ctx, tt.manifests)

			// Check error expectation
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, tsKinds)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, tsKinds)

			// Run validation
			if tt.validateTroubleshoot != nil {
				tt.validateTroubleshoot(t, tsKinds)
			}
		})
	}
}

// Helper function to create HelmChart from YAML string
func createHelmChartCRFromYAML(yamlStr string) *kotsv1beta2.HelmChart {
	var chart kotsv1beta2.HelmChart
	err := kyaml.Unmarshal([]byte(yamlStr), &chart)
	if err != nil {
		panic(err)
	}
	return &chart
}

// createComplexChartArchive creates a Helm chart archive with CRDs, deployment, and service
func createComplexChartArchive(t *testing.T, name, version string) []byte {
	chartYaml := fmt.Sprintf(`apiVersion: v2
name: %s
version: %s
description: A complex test Helm chart
type: application
`, name, version)

	crd := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
  namespace: {{ .Release.Namespace }}
spec:
  group: example.com
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
  scope: Namespaced
  names:
    plural: widgets
    singular: widget
    kind: Widget
`

	deployment := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "chart.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    app: {{ .Chart.Name }}
spec:
  replicas: {{ .Values.replicaCount | default 1 }}
  selector:
    matchLabels:
      app: {{ .Chart.Name }}
  template:
    metadata:
      labels:
        app: {{ .Chart.Name }}
    spec:
      containers:
      - name: {{ .Chart.Name }}
        image: {{ .Values.image.repository | default "nginx" }}:{{ .Values.image.tag | default "latest" }}
        ports:
        - containerPort: 80
`

	service := `apiVersion: v1
kind: Service
metadata:
  name: {{ include "chart.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    app: {{ .Chart.Name }}
spec:
  type: {{ .Values.service.type | default "ClusterIP" }}
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: {{ .Chart.Name }}
`

	helpers := `{{- define "chart.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
`

	valuesYaml := `replicaCount: 1

image:
  repository: nginx
  tag: latest
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 80

resources: {}
nodeSelector: {}
tolerations: []
affinity: {}
`

	files := map[string]string{
		fmt.Sprintf("%s/Chart.yaml", name):                chartYaml,
		fmt.Sprintf("%s/values.yaml", name):               valuesYaml,
		fmt.Sprintf("%s/templates/crd.yaml", name):        crd,
		fmt.Sprintf("%s/templates/deployment.yaml", name): deployment,
		fmt.Sprintf("%s/templates/service.yaml", name):    service,
		fmt.Sprintf("%s/templates/_helpers.tpl", name):    helpers,
	}

	return createTarGzArchive(t, files)
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
						// Additional items for ExtractAppPreflightSpec test
						{Name: "replica_count", Type: "text", Value: multitype.FromString("3")},
						{Name: "check_name", Type: "text", Value: multitype.FromString("K8s Version Validation")},
						{Name: "chart1_enabled", Type: "text", Value: multitype.FromString("true")},
						{Name: "node_count", Type: "text", Value: multitype.FromString("3")},
						{Name: "version_check_name", Type: "text", Value: multitype.FromString("Custom K8s Version Check")},
						{Name: "resource_check_name", Type: "text", Value: multitype.FromString("Custom Node Resource Check")},
					},
				},
			},
		},
	}
}
