package adminconsole

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateHelmValues_HostCABundlePath(t *testing.T) {
	t.Run("with host CA bundle path", func(t *testing.T) {
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())
		rc.SetHostCABundlePath("/etc/ssl/certs/ca-certificates.crt")

		addon := &AdminConsole{
			runtimeConfig: rc,
		}

		values, err := addon.GenerateHelmValues(context.Background(), types.InstallOptions{}, nil)
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
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		addon := &AdminConsole{
			runtimeConfig: rc,
		}

		values, err := addon.GenerateHelmValues(context.Background(), types.InstallOptions{}, nil)
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
