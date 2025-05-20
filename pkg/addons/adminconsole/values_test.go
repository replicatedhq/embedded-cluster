package adminconsole

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateHelmValues_HostCABundlePath(t *testing.T) {
	t.Run("with host CA bundle path", func(t *testing.T) {
		adminConsole := &AdminConsole{
			HostCABundlePath: "/etc/ssl/certs/ca-certificates.crt",
		}

		values, err := adminConsole.GenerateHelmValues(context.Background(), nil, nil)
		require.NoError(t, err, "GenerateHelmValues should not return an error")

		// Verify extraVolumes
		require.Contains(t, values, "extraVolumes", "Should have extraVolumes key")
		extraVolumes, ok := values["extraVolumes"].([]map[string]interface{})
		require.True(t, ok, "extraVolumes should be a slice of maps")
		require.Len(t, extraVolumes, 1, "Should have one volume")

		// Verify volume configuration
		volume := extraVolumes[0]
		assert.Equal(t, "host-ca-bundle", volume["name"], "Volume name should be host-ca-bundle")

		hostPath, ok := volume["hostPath"].(map[string]interface{})
		require.True(t, ok, "hostPath should be a map")
		assert.Equal(t, "/etc/ssl/certs/ca-certificates.crt", hostPath["path"], "Path should match HostCABundlePath")
		assert.Equal(t, "FileOrCreate", hostPath["type"], "Type should be FileOrCreate")

		// Verify extraVolumeMounts
		require.Contains(t, values, "extraVolumeMounts", "Should have extraVolumeMounts key")
		extraVolumeMounts, ok := values["extraVolumeMounts"].([]map[string]interface{})
		require.True(t, ok, "extraVolumeMounts should be a slice of maps")
		require.Len(t, extraVolumeMounts, 1, "Should have one volume mount")

		// Verify volume mount configuration
		volumeMount := extraVolumeMounts[0]
		assert.Equal(t, "host-ca-bundle", volumeMount["name"], "Volume mount name should be host-ca-bundle")
		assert.Equal(t, "/certs/ca-certificates.crt", volumeMount["mountPath"], "Mount path should be /certs/ca-certificates.crt")

		// Verify extraEnv
		require.Contains(t, values, "extraEnv", "Should have extraEnv key")
		extraEnv, ok := values["extraEnv"].([]map[string]interface{})
		require.True(t, ok, "extraEnv should be a slice of maps")

		// Find SSL_CERT_DIR environment variable
		var foundSSLCertDir bool
		for _, env := range extraEnv {
			if env["name"] == "SSL_CERT_DIR" {
				foundSSLCertDir = true
				assert.Equal(t, "/certs", env["value"], "SSL_CERT_DIR should be set to /certs")
				break
			}
		}
		assert.True(t, foundSSLCertDir, "Should have SSL_CERT_DIR environment variable")
	})

	t.Run("without host CA bundle path", func(t *testing.T) {
		adminConsole := &AdminConsole{
			// HostCABundlePath intentionally not set
		}

		values, err := adminConsole.GenerateHelmValues(context.Background(), nil, nil)
		require.NoError(t, err, "GenerateHelmValues should not return an error")

		// Verify extraVolumes is empty
		require.Contains(t, values, "extraVolumes", "Should have extraVolumes key")
		extraVolumes, ok := values["extraVolumes"].([]map[string]interface{})
		require.True(t, ok, "extraVolumes should be a slice of maps")
		assert.Empty(t, extraVolumes, "Should have no volumes")

		// Verify extraVolumeMounts is empty
		require.Contains(t, values, "extraVolumeMounts", "Should have extraVolumeMounts key")
		extraVolumeMounts, ok := values["extraVolumeMounts"].([]map[string]interface{})
		require.True(t, ok, "extraVolumeMounts should be a slice of maps")
		assert.Empty(t, extraVolumeMounts, "Should have no volume mounts")

		// Verify SSL_CERT_DIR is not in extraEnv
		require.Contains(t, values, "extraEnv", "Should have extraEnv key")
		extraEnv, ok := values["extraEnv"].([]map[string]interface{})
		require.True(t, ok, "extraEnv should be a slice of maps")

		for _, env := range extraEnv {
			assert.NotEqual(t, "SSL_CERT_DIR", env["name"], "Should not have SSL_CERT_DIR environment variable")
		}
	})
}
