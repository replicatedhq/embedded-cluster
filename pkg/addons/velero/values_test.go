package velero

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateHelmValues_HostCABundlePath(t *testing.T) {
	v := &Velero{
		HostCABundlePath: "/etc/ssl/certs/ca-certificates.crt",
	}

	rc := runtimeconfig.New(nil)

	values, err := v.GenerateHelmValues(context.Background(), nil, rc, nil)
	require.NoError(t, err, "GenerateHelmValues should not return an error")

	require.NotEmpty(t, values["extraVolumes"])
	require.IsType(t, []map[string]any{}, values["extraVolumes"])
	require.Len(t, values["extraVolumes"], 1)

	require.NotEmpty(t, values["extraVolumeMounts"])
	require.IsType(t, []map[string]any{}, values["extraVolumeMounts"])
	require.Len(t, values["extraVolumeMounts"], 1)

	require.IsType(t, map[string]any{}, values["configuration"])
	require.IsType(t, []map[string]any{}, values["configuration"].(map[string]any)["extraEnvVars"])

	require.IsType(t, map[string]any{}, values["nodeAgent"])
	require.IsType(t, []map[string]any{}, values["nodeAgent"].(map[string]any)["extraVolumes"])
	require.IsType(t, []map[string]any{}, values["nodeAgent"].(map[string]any)["extraVolumeMounts"])

	extraVolume := values["extraVolumes"].([]map[string]any)[0]
	if assert.NotNil(t, extraVolume["hostPath"]) {
		assert.Equal(t, "/etc/ssl/certs/ca-certificates.crt", extraVolume["hostPath"].(map[string]any)["path"])
		assert.Equal(t, "FileOrCreate", extraVolume["hostPath"].(map[string]any)["type"])
	}

	extraVolumeMount := values["extraVolumeMounts"].([]map[string]any)[0]
	assert.Equal(t, "host-ca-bundle", extraVolumeMount["name"])
	assert.Equal(t, "/certs/ca-certificates.crt", extraVolumeMount["mountPath"])

	extraEnvVars := values["configuration"].(map[string]any)["extraEnvVars"].([]map[string]any)
	// Find the SSL_CERT_DIR environment variable
	var foundSSLCertDir bool
	for _, env := range extraEnvVars {
		if env["name"] == "SSL_CERT_DIR" {
			assert.Equal(t, "/certs", env["value"])
			foundSSLCertDir = true
			break
		}
	}
	assert.True(t, foundSSLCertDir, "SSL_CERT_DIR environment variable should be set")

	extraVolumes := values["nodeAgent"].(map[string]any)["extraVolumes"].([]map[string]any)
	assert.Equal(t, "/etc/ssl/certs/ca-certificates.crt", extraVolumes[0]["hostPath"].(map[string]any)["path"])
	assert.Equal(t, "FileOrCreate", extraVolumes[0]["hostPath"].(map[string]any)["type"])

	extraVolumeMounts := values["nodeAgent"].(map[string]any)["extraVolumeMounts"].([]map[string]any)
	assert.Equal(t, "host-ca-bundle", extraVolumeMounts[0]["name"])
	assert.Equal(t, "/certs/ca-certificates.crt", extraVolumeMounts[0]["mountPath"])
}
