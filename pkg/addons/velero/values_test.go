package velero

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func TestGenerateHelmValues_HostCABundlePath(t *testing.T) {
	// Setup
	v := &Velero{
		HostCABundlePath: "/etc/ssl/certs/ca-certificates.crt",
	}

	// Create a fake k8s client
	kcli := fake.NewClientBuilder().Build()

	// Call function under test
	values, err := v.GenerateHelmValues(context.Background(), kcli, nil)
	require.NoError(t, err, "GenerateHelmValues should not return an error")

	assert.NotEmpty(t, values["extraVolumes"])
	assert.IsType(t, []string{}, values["extraVolumes"])
	assert.Len(t, values["extraVolumes"], 1)
	assert.NotEmpty(t, values["extraVolumeMounts"])
	assert.IsType(t, []string{}, values["extraVolumeMounts"])
	assert.Len(t, values["extraVolumeMounts"], 1)

	var extraVolume corev1.Volume
	err = yaml.Unmarshal([]byte(values["extraVolumes"].([]string)[0]), &extraVolume)
	require.NoError(t, err, "Failed to unmarshal extraVolumes")

	assert.NotNil(t, extraVolume.HostPath)
	assert.Equal(t, "/etc/ssl/certs/ca-certificates.crt", extraVolume.HostPath.Path)
	assert.Equal(t, corev1.HostPathFileOrCreate, *extraVolume.HostPath.Type)

	var extraVolumeMount corev1.VolumeMount
	err = yaml.Unmarshal([]byte(values["extraVolumeMounts"].([]string)[0]), &extraVolumeMount)
	require.NoError(t, err, "Failed to unmarshal extraVolumeMounts")

	assert.Equal(t, "host-ca-bundle", extraVolumeMount.Name)
	assert.Equal(t, "/certs/ca-certificates.crt", extraVolumeMount.MountPath)

	configuration, ok := values["configuration"].(map[string]any)
	require.True(t, ok, "configuration should be a map")

	extraEnvVars, ok := configuration["extraEnvVars"].(map[string]any)
	require.True(t, ok, "extraEnvVars should be a map")

	assert.Equal(t, "/certs", extraEnvVars["SSL_CERT_DIR"])
}
