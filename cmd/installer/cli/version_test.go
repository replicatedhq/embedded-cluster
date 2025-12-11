package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/stretchr/testify/assert"
)

func TestCollectBinaryVersions(t *testing.T) {
	tests := []struct {
		name               string
		channelRelease     *release.ChannelRelease
		expectedAppVersion string
		expectAppInOrder   bool
	}{
		{
			name: "with channel release",
			channelRelease: &release.ChannelRelease{
				VersionLabel: "v1.2.3",
			},
			expectedAppVersion: "v1.2.3",
			expectAppInOrder:   true,
		},
		{
			name:             "without channel release",
			channelRelease:   nil,
			expectAppInOrder: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			componentVersions, orderedKeys := collectBinaryVersions(tt.channelRelease)

			// Verify Installer and Kubernetes are always present
			assert.Contains(t, componentVersions, "Installer")
			assert.Contains(t, componentVersions, "Kubernetes (bundled)")

			// Verify app version if channel release is provided
			if tt.channelRelease != nil {
				// First key should be the app slug
				assert.Greater(t, len(orderedKeys), 0)
				appSlug := orderedKeys[0]
				assert.Contains(t, componentVersions, appSlug)
				assert.Equal(t, tt.expectedAppVersion, componentVersions[appSlug])

				// Verify order: app, Installer, Kubernetes (bundled), then addons
				assert.GreaterOrEqual(t, len(orderedKeys), 3)
				assert.Equal(t, "Installer", orderedKeys[1])
				assert.Equal(t, "Kubernetes (bundled)", orderedKeys[2])
			} else {
				// Without channel release, first should be Installer
				assert.Greater(t, len(orderedKeys), 0)
				assert.Equal(t, "Installer", orderedKeys[0])
				assert.Equal(t, "Kubernetes (bundled)", orderedKeys[1])
			}

			// Verify all keys in map are in ordered list
			assert.Len(t, orderedKeys, len(componentVersions))
		})
	}
}

func TestCollectAndNormalizeVersions(t *testing.T) {
	tests := []struct {
		name           string
		source         map[string]string
		expectedTarget map[string]string
		expectedKeys   []string
	}{
		{
			name: "versions with v prefix",
			source: map[string]string{
				"component1": "v1.2.3",
				"component2": "v2.0.0",
			},
			expectedTarget: map[string]string{
				"component1": "v1.2.3",
				"component2": "v2.0.0",
			},
			expectedKeys: []string{"component1", "component2"},
		},
		{
			name: "versions without v prefix",
			source: map[string]string{
				"component1": "1.2.3",
				"component2": "2.0.0",
			},
			expectedTarget: map[string]string{
				"component1": "v1.2.3",
				"component2": "v2.0.0",
			},
			expectedKeys: []string{"component1", "component2"},
		},
		{
			name: "mixed versions",
			source: map[string]string{
				"component1": "v1.2.3",
				"component2": "2.0.0",
			},
			expectedTarget: map[string]string{
				"component1": "v1.2.3",
				"component2": "v2.0.0",
			},
			expectedKeys: []string{"component1", "component2"},
		},
		{
			name:           "empty source",
			source:         map[string]string{},
			expectedTarget: map[string]string{},
			expectedKeys:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := make(map[string]string)
			keys := []string{}

			collectAndNormalizeVersions(tt.source, target, &keys)

			// Verify all expected keys are present in target
			for k, v := range tt.expectedTarget {
				assert.Contains(t, target, k)
				assert.Equal(t, v, target[k])
			}

			// Verify keys slice has correct length
			assert.Len(t, keys, len(tt.expectedKeys))

			// Verify all keys are present (order may vary due to map iteration)
			for _, expectedKey := range tt.expectedKeys {
				assert.Contains(t, keys, expectedKey)
			}
		})
	}
}

func TestPrintVersionSection(t *testing.T) {
	tests := []struct {
		name              string
		header            string
		componentVersions map[string]string
		orderedKeys       []string
		expectedContains  []string
	}{
		{
			name:   "with ordered keys",
			header: "CLIENT (Binary)",
			componentVersions: map[string]string{
				"app":        "v1.0.0",
				"Installer":  "v2.0.0",
				"Kubernetes": "v1.28.0",
			},
			orderedKeys: []string{"app", "Installer", "Kubernetes"},
			expectedContains: []string{
				"CLIENT (Binary)",
				"---------------",
				"app",
				"v1.0.0",
				"Installer",
				"v2.0.0",
				"Kubernetes",
				"v1.28.0",
			},
		},
		{
			name:   "without ordered keys (alphabetical)",
			header: "SERVER (Deployed)",
			componentVersions: map[string]string{
				"zebra": "v1.0.0",
				"alpha": "v2.0.0",
				"beta":  "v3.0.0",
			},
			orderedKeys: nil,
			expectedContains: []string{
				"SERVER (Deployed)",
				"-----------------",
				"alpha",
				"v2.0.0",
				"beta",
				"v3.0.0",
				"zebra",
				"v1.0.0",
			},
		},
		{
			name:              "empty versions",
			header:            "EMPTY",
			componentVersions: map[string]string{},
			orderedKeys:       []string{},
			expectedContains: []string{
				"EMPTY",
				"-----",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			printVersionSection(tt.header, tt.componentVersions, tt.orderedKeys)

			w.Close()
			os.Stdout = old

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			// Verify all expected strings are present
			for _, expected := range tt.expectedContains {
				assert.Contains(t, output, expected, "output should contain %q", expected)
			}

			// Verify header underline length matches header
			lines := strings.Split(output, "\n")
			if len(lines) >= 2 {
				assert.Equal(t, len(tt.header), len(strings.TrimSpace(lines[1])), "underline should match header length")
			}

			// Verify proper indentation (2 spaces at start of component lines)
			for _, line := range lines[2:] {
				if strings.TrimSpace(line) != "" {
					assert.True(t, strings.HasPrefix(line, "  "), "component lines should start with 2 spaces")
				}
			}
		})
	}
}

func TestPrintVersionSectionSpacing(t *testing.T) {
	componentVersions := map[string]string{
		"short":       "v1.0.0",
		"much-longer": "v2.0.0",
		"x":           "v3.0.0",
	}
	orderedKeys := []string{"short", "much-longer", "x"}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printVersionSection("TEST", componentVersions, orderedKeys)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	lines := strings.Split(output, "\n")

	// Find component lines (skip header and underline)
	var componentLines []string
	for i := 2; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			componentLines = append(componentLines, lines[i])
		}
	}

	// Verify spacing between component name and version
	// Should be at least 2 spaces between longest name and its version
	for _, line := range componentLines {
		// Split on multiple spaces to find the gap
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			// Find the position of the version
			versionIdx := strings.Index(line, parts[len(parts)-1])
			// Find the position after the component name
			componentName := strings.Join(parts[:len(parts)-1], " ")
			componentEndIdx := strings.Index(line, componentName) + len(componentName)

			// Calculate spacing
			spacing := versionIdx - componentEndIdx
			assert.GreaterOrEqual(t, spacing, 2, "spacing should be at least 2 spaces")
		}
	}
}

func TestCollectDeployedVersions(t *testing.T) {
	// Test that when no cluster is accessible, we return empty map and false
	versions, hasCluster := collectDeployedVersions(context.Background())

	// In test environment without a cluster, should return false
	assert.False(t, hasCluster)
	// Should return empty map
	assert.Empty(t, versions)
}
