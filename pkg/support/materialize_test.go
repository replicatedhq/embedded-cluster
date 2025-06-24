package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaterializeSupportBundleSpec(t *testing.T) {
	tests := []struct {
		name           string
		isAirgap       bool
		proxySpec      *ecv1beta1.ProxySpec
		expectedInFile []string
		notInFile      []string
		validateFunc   func(t *testing.T, content string)
	}{
		{
			name:     "airgap installation - HTTP collectors excluded",
			isAirgap: true,
			proxySpec: &ecv1beta1.ProxySpec{
				HTTPSProxy: "https://proxy:8080",
				HTTPProxy:  "http://proxy:8080",
				NoProxy:    "localhost,127.0.0.1",
			},
			expectedInFile: []string{
				// Core collectors should always be present
				"k8s-api-healthz-6443",
				"free",
				"embedded-cluster-path-usage",
				// HTTP collectors are present in template (but will be excluded)
				"http-replicated-app",
				"curl-replicated-app",
			},
			notInFile: []string{
				// Template variables should be substituted
				"{{ .ReplicatedAppURL }}",
				"{{ .ProxyRegistryURL }}",
				"{{ .HTTPSProxy }}",
			},
			validateFunc: func(t *testing.T, content string) {
				// Validate that HTTP collectors have exclude: 'true' for airgap
				assert.Contains(t, content, "collectorName: http-replicated-app")

				// Check that the http-replicated-app collector block has exclude: 'true'
				httpCollectorStart := strings.Index(content, "collectorName: http-replicated-app")
				require.Greater(t, httpCollectorStart, -1, "http-replicated-app collector should be present")

				// Find the next collector to limit our search scope
				nextCollectorStart := strings.Index(content[httpCollectorStart+1:], "collectorName:")
				var httpCollectorBlock string
				if nextCollectorStart > -1 {
					httpCollectorBlock = content[httpCollectorStart : httpCollectorStart+1+nextCollectorStart]
				} else {
					httpCollectorBlock = content[httpCollectorStart:]
				}

				assert.Contains(t, httpCollectorBlock, "exclude: 'true'",
					"http-replicated-app collector should be excluded in airgap mode")

				// Also validate curl-replicated-app is excluded
				curlCollectorStart := strings.Index(content, "collectorName: curl-replicated-app")
				require.Greater(t, curlCollectorStart, -1, "curl-replicated-app collector should be present")

				nextCurlCollectorStart := strings.Index(content[curlCollectorStart+1:], "collectorName:")
				var curlCollectorBlock string
				if nextCurlCollectorStart > -1 {
					curlCollectorBlock = content[curlCollectorStart : curlCollectorStart+1+nextCurlCollectorStart]
				} else {
					curlCollectorBlock = content[curlCollectorStart:]
				}

				assert.Contains(t, curlCollectorBlock, "exclude: 'true'",
					"curl-replicated-app collector should be excluded in airgap mode")
			},
		},
		{
			name:     "online installation with proxy - HTTP collectors included",
			isAirgap: false,
			proxySpec: &ecv1beta1.ProxySpec{
				HTTPSProxy: "https://proxy:8080",
				HTTPProxy:  "http://proxy:8080",
				NoProxy:    "localhost,127.0.0.1",
			},
			expectedInFile: []string{
				// Core collectors
				"k8s-api-healthz-6443",
				"free",
				"embedded-cluster-path-usage",
				// HTTP collectors are included for online
				"http-replicated-app",
				"curl-replicated-app",
				// URLs and proxy settings
				"https://replicated.app/healthz",
				"https://proxy.replicated.com/v2/",
				"proxy: 'https://proxy:8080'",
			},
			notInFile: []string{
				// Template variables should be substituted
				"{{ .ReplicatedAppURL }}",
				"{{ .HTTPSProxy }}",
			},
			validateFunc: func(t *testing.T, content string) {
				// Validate that HTTP collectors have exclude: 'false' for online
				assert.Contains(t, content, "collectorName: http-replicated-app")

				// Check that the http-replicated-app collector block has exclude: 'false'
				httpCollectorStart := strings.Index(content, "collectorName: http-replicated-app")
				require.Greater(t, httpCollectorStart, -1, "http-replicated-app collector should be present")

				// Find the next collector to limit our search scope
				nextCollectorStart := strings.Index(content[httpCollectorStart+1:], "collectorName:")
				var httpCollectorBlock string
				if nextCollectorStart > -1 {
					httpCollectorBlock = content[httpCollectorStart : httpCollectorStart+1+nextCollectorStart]
				} else {
					httpCollectorBlock = content[httpCollectorStart:]
				}

				assert.Contains(t, httpCollectorBlock, "exclude: 'false'",
					"http-replicated-app collector should not be excluded in online mode")
			},
		},
		{
			name:      "online installation without proxy - HTTP collectors included, no proxy config",
			isAirgap:  false,
			proxySpec: nil,
			expectedInFile: []string{
				// Core collectors
				"k8s-api-healthz-6443",
				"embedded-cluster-path-usage",
				// HTTP collectors included
				"http-replicated-app",
				"curl-replicated-app",
				// URLs populated
				"https://replicated.app/healthz",
				"https://proxy.replicated.com/v2/",
			},
			notInFile: []string{
				// No proxy settings when proxy not configured
				"proxy: 'https://proxy:8080'",
				"proxy: 'http://proxy:8080'",
				// Template variables should be substituted
				"{{ .HTTPSProxy }}",
				"{{ .HTTPProxy }}",
			},
			validateFunc: func(t *testing.T, content string) {
				// Validate that HTTP collectors have exclude: 'false' for online
				httpCollectorStart := strings.Index(content, "collectorName: http-replicated-app")
				require.Greater(t, httpCollectorStart, -1, "http-replicated-app collector should be present")

				nextCollectorStart := strings.Index(content[httpCollectorStart+1:], "collectorName:")
				var httpCollectorBlock string
				if nextCollectorStart > -1 {
					httpCollectorBlock = content[httpCollectorStart : httpCollectorStart+1+nextCollectorStart]
				} else {
					httpCollectorBlock = content[httpCollectorStart:]
				}

				assert.Contains(t, httpCollectorBlock, "exclude: 'false'",
					"http-replicated-app collector should not be excluded in online mode")

				// Verify proxy is empty/not set in the collector block
				assert.Contains(t, httpCollectorBlock, "proxy: ''",
					"proxy should be empty when no proxy is configured")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for the test
			tempDir := t.TempDir()

			// Create the support subdirectory
			supportDir := filepath.Join(tempDir, "support")
			err := os.MkdirAll(supportDir, 0755)
			require.NoError(t, err)

			// Copy the actual customer template to the test directory
			actualTemplatePath := filepath.Join("../../cmd/installer/goods/support/host-support-bundle.tmpl.yaml")
			templateContent, err := os.ReadFile(actualTemplatePath)
			require.NoError(t, err, "Should be able to read the actual customer template")

			// Write the actual template to the test directory
			templatePath := filepath.Join(supportDir, "host-support-bundle.tmpl.yaml")
			err = os.WriteFile(templatePath, templateContent, 0644)
			require.NoError(t, err)

			// Create mock RuntimeConfig
			mockRC := &runtimeconfig.MockRuntimeConfig{}
			mockRC.On("EmbeddedClusterHomeDirectory").Return(tempDir)
			mockRC.On("EmbeddedClusterK0sSubDir").Return(filepath.Join(tempDir, "k0s"))
			mockRC.On("EmbeddedClusterOpenEBSLocalSubDir").Return(filepath.Join(tempDir, "openebs"))
			mockRC.On("PathToEmbeddedClusterSupportFile", "host-support-bundle.tmpl.yaml").Return(templatePath)
			mockRC.On("PathToEmbeddedClusterSupportFile", "host-support-bundle.yaml").Return(
				filepath.Join(supportDir, "host-support-bundle.yaml"))
			mockRC.On("ProxySpec").Return(tt.proxySpec)

			// Call the function under test
			err = MaterializeSupportBundleSpec(mockRC, tt.isAirgap)
			require.NoError(t, err)

			// Verify the file was created
			outputFile := filepath.Join(supportDir, "host-support-bundle.yaml")
			_, err = os.Stat(outputFile)
			require.NoError(t, err, "Support bundle spec file should be created")

			// Read the generated file content
			content, err := os.ReadFile(outputFile)
			require.NoError(t, err)
			contentStr := string(content)

			// Verify expected content is present
			for _, expected := range tt.expectedInFile {
				assert.Contains(t, contentStr, expected,
					"Expected %q to be in the generated support bundle spec", expected)
			}

			// Verify unwanted content is not present
			for _, notExpected := range tt.notInFile {
				assert.NotContains(t, contentStr, notExpected,
					"Expected %q to NOT be in the generated support bundle spec", notExpected)
			}

			// Verify that key template variables were properly substituted
			assert.Contains(t, contentStr, tempDir, "Data directory should be substituted")
			assert.Contains(t, contentStr, filepath.Join(tempDir, "k0s"), "K0s data directory should be substituted")
			assert.Contains(t, contentStr, filepath.Join(tempDir, "openebs"), "OpenEBS data directory should be substituted")

			// Verify the YAML structure is valid
			assert.Contains(t, contentStr, "apiVersion: troubleshoot.sh/v1beta2")
			assert.Contains(t, contentStr, "kind: SupportBundle")
			assert.Contains(t, contentStr, "hostCollectors:")
			assert.Contains(t, contentStr, "hostAnalyzers:")

			// Verify key collectors that should always be present
			assert.Contains(t, contentStr, "ipv4Interfaces", "Basic network collector should be present")
			assert.Contains(t, contentStr, "memory", "Memory collector should be present")
			assert.Contains(t, contentStr, "filesystem-write-latency-etcd", "Performance collector should be present")

			// Run the specific validation function for this test case
			if tt.validateFunc != nil {
				tt.validateFunc(t, contentStr)
			}

			// Assert all mock expectations were met
			mockRC.AssertExpectations(t)
		})
	}
}
