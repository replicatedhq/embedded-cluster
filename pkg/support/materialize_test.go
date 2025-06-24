package support

import (
	"os"
	"path/filepath"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaterializeSupportBundleSpec(t *testing.T) {
	// Create a test template that includes the conditional logic we want to test
	testTemplate := `apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: test-support-bundle
spec:
  hostCollectors:
  - diskUsage:
      collectorName: embedded-cluster-path-usage
      path: {{ .DataDir }}
  - diskUsage:
      collectorName: k0s-path-usage
      path: {{ .K0sDataDir }}
  - diskUsage:
      collectorName: openebs-path-usage
      path: {{ .OpenEBSDataDir }}
{{- if .IsAirgap }}
  # Airgap-specific collectors
  - run:
      collectorName: airgap-test-collector
      command: echo
      args: ["airgap mode enabled"]
  - run:
      collectorName: offline-check
      command: test
      args: ["-f", "{{ .DataDir }}/offline-mode"]
  - run:
      collectorName: local-service-status
      command: systemctl
      args: ["status", "local-artifact-mirror.service"]
{{- else }}
  # Online installation collectors
  - run:
      collectorName: online-test-collector
      command: echo
      args: ["online mode enabled"]
  - run:
      collectorName: internet-check
      command: ping
      args: ["-c", "1", "example.com"]
  - http:
      collectorName: http-replicated-app
      get:
        url: '{{ .ReplicatedAppURL }}/healthz'
        timeout: 5s
        proxy: '{{ .HTTPSProxy }}'
      exclude: '{{ eq .ReplicatedAppURL "" }}'
{{- if .HTTPSProxy }}
  - run:
      collectorName: proxy-test-collector
      command: curl
      args: ["--proxy", "{{ .HTTPSProxy }}", "{{ .ReplicatedAppURL }}"]
      exclude: '{{ eq .ReplicatedAppURL "" }}'
{{- else }}
  - run:
      collectorName: no-proxy-collector
      command: curl
      args: ["{{ .ReplicatedAppURL }}"]
      exclude: '{{ eq .ReplicatedAppURL "" }}'
{{- end }}
{{- end }}`

	tests := []struct {
		name           string
		isAirgap       bool
		proxySpec      *ecv1beta1.ProxySpec
		expectedInFile []string
		notInFile      []string
	}{
		{
			name:     "airgap installation",
			isAirgap: true,
			proxySpec: &ecv1beta1.ProxySpec{
				HTTPSProxy: "https://proxy:8080",
			},
			expectedInFile: []string{
				"airgap-test-collector",
				"offline-check",
				"local-service-status",
			},
			notInFile: []string{
				"online-test-collector",
				"internet-check",
				"proxy-test-collector",
			},
		},
		{
			name:     "online installation with proxy",
			isAirgap: false,
			proxySpec: &ecv1beta1.ProxySpec{
				HTTPSProxy: "https://proxy:8080",
			},
			expectedInFile: []string{
				"online-test-collector",
				"internet-check",
				"proxy-test-collector",
				"--proxy", "https://proxy:8080",
			},
			notInFile: []string{
				"airgap-test-collector",
				"offline-check",
				"no-proxy-collector",
			},
		},
		{
			name:      "online installation without proxy",
			isAirgap:  false,
			proxySpec: nil,
			expectedInFile: []string{
				"online-test-collector",
				"internet-check",
				"no-proxy-collector",
			},
			notInFile: []string{
				"airgap-test-collector",
				"offline-check",
				"proxy-test-collector",
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

			// Create the template file
			templatePath := filepath.Join(supportDir, "host-support-bundle.tmpl.yaml")
			err = os.WriteFile(templatePath, []byte(testTemplate), 0644)
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

			// Verify that template variables were properly substituted
			assert.Contains(t, contentStr, tempDir, "Data directory should be substituted")
			assert.Contains(t, contentStr, filepath.Join(tempDir, "k0s"), "K0s data directory should be substituted")
			assert.Contains(t, contentStr, filepath.Join(tempDir, "openebs"), "OpenEBS data directory should be substituted")

			// Verify no template variables remain unsubstituted
			assert.NotContains(t, contentStr, "{{ .", "No template variables should remain unsubstituted")

			// Assert all mock expectations were met
			mockRC.AssertExpectations(t)
		})
	}
}
