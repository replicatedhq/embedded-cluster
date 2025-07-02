package adminconsole

import (
	"context"
	"path/filepath"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateHelmValues_HostCABundlePath(t *testing.T) {
	t.Run("with host CA bundle path", func(t *testing.T) {
		adminConsole := &AdminConsole{
			DataDir:          t.TempDir(),
			HostCABundlePath: "/etc/ssl/certs/ca-certificates.crt",
		}

		values, err := adminConsole.GenerateHelmValues(context.Background(), nil, ecv1beta1.Domains{}, nil)
		require.NoError(t, err, "GenerateHelmValues should not return an error")

		// Verify structure types
		require.NotEmpty(t, values["extraVolumes"])
		require.IsType(t, []map[string]interface{}{}, values["extraVolumes"])
		require.Len(t, values["extraVolumes"].([]map[string]interface{}), 1)

		require.NotEmpty(t, values["extraVolumeMounts"])
		require.IsType(t, []map[string]interface{}{}, values["extraVolumeMounts"])
		require.Len(t, values["extraVolumeMounts"].([]map[string]interface{}), 1)

		require.NotEmpty(t, values["extraEnv"])
		require.IsType(t, []map[string]interface{}{}, values["extraEnv"])

		// Verify volume configuration
		extraVolume := values["extraVolumes"].([]map[string]interface{})[0]
		assert.Equal(t, "host-ca-bundle", extraVolume["name"])
		if assert.NotNil(t, extraVolume["hostPath"]) {
			hostPath := extraVolume["hostPath"].(map[string]interface{})
			assert.Equal(t, "/etc/ssl/certs/ca-certificates.crt", hostPath["path"])
			assert.Equal(t, "FileOrCreate", hostPath["type"])
		}

		// Verify volume mount configuration
		extraVolumeMount := values["extraVolumeMounts"].([]map[string]interface{})[0]
		assert.Equal(t, "host-ca-bundle", extraVolumeMount["name"])
		assert.Equal(t, "/certs/ca-certificates.crt", extraVolumeMount["mountPath"])

		// Verify SSL_CERT_DIR environment variable
		extraEnv := values["extraEnv"].([]map[string]interface{})
		var foundSSLCertDir bool
		for _, env := range extraEnv {
			if env["name"] == "SSL_CERT_DIR" {
				foundSSLCertDir = true
				assert.Equal(t, "/certs", env["value"])
				break
			}
		}
		assert.True(t, foundSSLCertDir, "SSL_CERT_DIR environment variable should be set")
	})

	t.Run("without host CA bundle path", func(t *testing.T) {
		// HostCABundlePath intentionally not set
		adminConsole := &AdminConsole{
			DataDir: t.TempDir(),
		}

		values, err := adminConsole.GenerateHelmValues(context.Background(), nil, ecv1beta1.Domains{}, nil)
		require.NoError(t, err, "GenerateHelmValues should not return an error")

		// Verify structure types
		require.IsType(t, []map[string]interface{}{}, values["extraVolumes"])
		require.Len(t, values["extraVolumes"].([]map[string]interface{}), 0)

		require.IsType(t, []map[string]interface{}{}, values["extraVolumeMounts"])
		require.Len(t, values["extraVolumeMounts"].([]map[string]interface{}), 0)

		require.IsType(t, []map[string]interface{}{}, values["extraEnv"])

		// Verify SSL_CERT_DIR is not present in any environment variable
		extraEnv := values["extraEnv"].([]map[string]interface{})
		for _, env := range extraEnv {
			assert.NotEqual(t, "SSL_CERT_DIR", env["name"], "SSL_CERT_DIR environment variable should not be set")
		}
	})
}

func TestGenerateHelmValues_Target(t *testing.T) {
	t.Run("Linux (with cluster ID)", func(t *testing.T) {
		dataDir := t.TempDir()

		adminConsole := &AdminConsole{
			IsAirgap:           false,
			IsHA:               false,
			IsMultiNodeEnabled: false,
			Proxy:              nil,
			AdminConsolePort:   8080,

			ClusterID:        "123",
			ServiceCIDR:      "10.0.0.0/24",
			HostCABundlePath: "/etc/ssl/certs/ca-certificates.crt",
			DataDir:          dataDir,
			K0sDataDir:       filepath.Join(dataDir, "k0s"),
		}

		values, err := adminConsole.GenerateHelmValues(context.Background(), nil, ecv1beta1.Domains{}, nil)
		require.NoError(t, err, "GenerateHelmValues should not return an error")

		assert.Contains(t, values, "embeddedClusterID")
		assert.Equal(t, "123", values["embeddedClusterID"])
		assert.Equal(t, dataDir, values["embeddedClusterDataDir"])
		assert.Equal(t, filepath.Join(dataDir, "k0s"), values["embeddedClusterK0sDir"])

		assert.Contains(t, values["extraEnv"], map[string]interface{}{
			"name":  "ENABLE_IMPROVED_DR",
			"value": "true",
		})
	})

	t.Run("Kubernetes (without cluster ID)", func(t *testing.T) {
		adminConsole := &AdminConsole{
			IsAirgap:           false,
			IsHA:               false,
			IsMultiNodeEnabled: false,
			Proxy:              nil,
			AdminConsolePort:   8080,
		}

		values, err := adminConsole.GenerateHelmValues(context.Background(), nil, ecv1beta1.Domains{}, nil)
		require.NoError(t, err, "GenerateHelmValues should not return an error")

		assert.NotContains(t, values, "embeddedClusterID")
		assert.NotContains(t, values, "embeddedClusterDataDir")
		assert.NotContains(t, values, "embeddedClusterK0sDir")

		for _, env := range values["extraEnv"].([]map[string]interface{}) {
			assert.NotEqual(t, "ENABLE_IMPROVED_DR", env["name"], "ENABLE_IMPROVED_DR environment variable should not be set")
		}
	})
}
