package embeddedclusteroperator

import (
	"context"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateHelmValues_HostCABundlePath(t *testing.T) {
	e := &EmbeddedClusterOperator{
		ClusterID:        "123",
		HostCABundlePath: "/etc/ssl/certs/ca-certificates.crt",
	}

	values, err := e.GenerateHelmValues(context.Background(), nil, ecv1beta1.Domains{}, nil)
	require.NoError(t, err, "GenerateHelmValues should not return an error")

	require.NotEmpty(t, values["extraVolumes"])
	require.IsType(t, []map[string]any{}, values["extraVolumes"])
	require.Len(t, values["extraVolumes"], 1)

	require.NotEmpty(t, values["extraVolumeMounts"])
	require.IsType(t, []map[string]any{}, values["extraVolumeMounts"])
	require.Len(t, values["extraVolumeMounts"], 1)

	require.NotEmpty(t, values["extraEnv"])
	require.IsType(t, []map[string]any{}, values["extraEnv"])

	// Find the SSL_CERT_DIR environment variable
	var sslCertDirFound bool
	for _, env := range values["extraEnv"].([]map[string]any) {
		if env["name"] == "SSL_CERT_DIR" {
			assert.Equal(t, "/certs", env["value"])
			sslCertDirFound = true
			break
		}
	}
	assert.True(t, sslCertDirFound, "SSL_CERT_DIR environment variable should be present")

	extraVolume := values["extraVolumes"].([]map[string]any)[0]
	assert.Equal(t, "host-ca-bundle", extraVolume["name"])
	if assert.NotNil(t, extraVolume["hostPath"]) {
		assert.Equal(t, "/etc/ssl/certs/ca-certificates.crt", extraVolume["hostPath"].(map[string]any)["path"])
		assert.Equal(t, "FileOrCreate", extraVolume["hostPath"].(map[string]any)["type"])
	}

	extraVolumeMount := values["extraVolumeMounts"].([]map[string]any)[0]
	assert.Equal(t, "host-ca-bundle", extraVolumeMount["name"])
	assert.Equal(t, "/certs/ca-certificates.crt", extraVolumeMount["mountPath"])

	assert.Equal(t, "123", values["embeddedClusterID"])
}
