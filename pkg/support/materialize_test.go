package support

import (
	"strings"
	"testing"
)

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		data     TemplateData
		want     []string
		dontWant []string
	}{
		{
			name: "airgap installation",
			data: TemplateData{
				IsAirgap:   true,
				HTTPSProxy: "https://proxy:8080",
			},
			want: []string{
				"airgap-test-collector",
				"offline-check",
				"local-service-status",
			},
			dontWant: []string{
				"online-test-collector",
				"internet-check",
				"proxy-test-collector",
			},
		},
		{
			name: "online installation with proxy",
			data: TemplateData{
				IsAirgap:         false,
				HTTPSProxy:       "https://proxy:8080",
				ReplicatedAppURL: "https://api.replicated.com",
			},
			want: []string{
				"online-test-collector",
				"internet-check",
				"proxy-test-collector",
				"--proxy", "https://proxy:8080",
			},
			dontWant: []string{
				"airgap-test-collector",
				"offline-check",
				"no-proxy-collector",
			},
		},
		{
			name: "online installation without proxy",
			data: TemplateData{
				IsAirgap:         false,
				HTTPSProxy:       "",
				ReplicatedAppURL: "https://api.replicated.com",
			},
			want: []string{
				"online-test-collector",
				"internet-check",
				"no-proxy-collector",
			},
			dontWant: []string{
				"airgap-test-collector",
				"offline-check",
				"proxy-test-collector",
			},
		},
	}

	tmpl := `{{- if .IsAirgap }}
# Airgap-specific test collectors
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
    args: ["status", "local-test-service"]
{{- else }}
# Online installation test collectors
- run:
    collectorName: online-test-collector
    command: echo
    args: ["online mode enabled"]
- run:
    collectorName: internet-check
    command: ping
    args: ["-c", "1", "example.com"]
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set default test data paths
			tt.data.DataDir = "/test/data"
			tt.data.K0sDataDir = "/test/k0s"
			tt.data.OpenEBSDataDir = "/test/openebs"

			result, err := renderTemplate(tmpl, tt.data)
			if err != nil {
				t.Fatalf("renderTemplate() error = %v", err)
			}

			// Check that wanted strings are present
			for _, want := range tt.want {
				if !strings.Contains(result, want) {
					t.Errorf("Expected result to contain %q, but it didn't. Result:\n%s", want, result)
				}
			}

			// Check that unwanted strings are not present
			for _, dontWant := range tt.dontWant {
				if strings.Contains(result, dontWant) {
					t.Errorf("Expected result to NOT contain %q, but it did. Result:\n%s", dontWant, result)
				}
			}
		})
	}
}

func TestTemplateDataFields(t *testing.T) {
	data := TemplateData{
		DataDir:          "/test/data",
		K0sDataDir:       "/test/k0s",
		OpenEBSDataDir:   "/test/openebs",
		IsAirgap:         true,
		ReplicatedAppURL: "https://api.replicated.com",
		ProxyRegistryURL: "https://proxy.replicated.com",
		HTTPSProxy:       "https://proxy:8080",
	}

	tmpl := `DataDir: {{ .DataDir }}
K0sDataDir: {{ .K0sDataDir }}
OpenEBSDataDir: {{ .OpenEBSDataDir }}
IsAirgap: {{ .IsAirgap }}
ReplicatedAppURL: {{ .ReplicatedAppURL }}
ProxyRegistryURL: {{ .ProxyRegistryURL }}
HTTPSProxy: {{ .HTTPSProxy }}`

	result, err := renderTemplate(tmpl, data)
	if err != nil {
		t.Fatalf("renderTemplate() error = %v", err)
	}

	expected := []string{
		"DataDir: /test/data",
		"K0sDataDir: /test/k0s",
		"OpenEBSDataDir: /test/openebs",
		"IsAirgap: true",
		"ReplicatedAppURL: https://api.replicated.com",
		"ProxyRegistryURL: https://proxy.replicated.com",
		"HTTPSProxy: https://proxy:8080",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("Expected result to contain %q, but it didn't. Result:\n%s", exp, result)
		}
	}
}
