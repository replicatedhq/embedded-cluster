package support

import (
	"strings"
	"testing"
)

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		isAirgap bool
		want     []string
		dontWant []string
	}{
		{
			name:     "airgap installation",
			isAirgap: true,
			want: []string{
				"local-artifact-mirror-status",
				"airgap-artifacts",
				"airgap-registry-check",
			},
			dontWant: []string{
				"internet-connectivity-check",
				"dns-resolution-check",
			},
		},
		{
			name:     "online installation",
			isAirgap: false,
			want: []string{
				"internet-connectivity-check",
				"dns-resolution-check",
			},
			dontWant: []string{
				"local-artifact-mirror-status",
				"airgap-artifacts",
				"airgap-registry-check",
			},
		},
	}

	tmpl := `{{- if .IsAirgap }}
# Airgap-specific collectors
- run:
    collectorName: local-artifact-mirror-status
    command: systemctl
    args: ["status", "local-artifact-mirror"]
- copy:
    collectorName: airgap-artifacts
    path: {{ .DataDir }}/artifacts/*
- run:
    collectorName: airgap-registry-check
    command: sh
    args: ["-c", "find {{ .DataDir }} -name '*registry*' -o -name '*airgap*' | head -20"]
{{- else }}
# Online installation collectors
- run:
    collectorName: internet-connectivity-check
    command: curl
    args: ["--connect-timeout", "5", "--max-time", "10", "-I", "https://proxy.replicated.com"]
- run:
    collectorName: dns-resolution-check
    command: nslookup
    args: ["proxy.replicated.com"]
{{- end }}`

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := TemplateData{
				DataDir:        "/var/lib/embedded-cluster",
				K0sDataDir:     "/var/lib/k0s",
				OpenEBSDataDir: "/var/openebs/local",
				IsAirgap:       tt.isAirgap,
			}

			result, err := renderTemplate(tmpl, data)
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
