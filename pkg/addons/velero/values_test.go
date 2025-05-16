package velero

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

	require.NotEmpty(t, values["extraVolumes"])
	require.IsType(t, []corev1.Volume{}, values["extraVolumes"])
	require.Len(t, values["extraVolumes"], 1)

	require.NotEmpty(t, values["extraVolumeMounts"])
	require.IsType(t, []corev1.VolumeMount{}, values["extraVolumeMounts"])
	require.Len(t, values["extraVolumeMounts"], 1)

	require.IsType(t, map[string]any{}, values["configuration"])
	require.IsType(t, map[string]any{}, values["configuration"].(map[string]any)["extraEnvVars"])

	extraVolume := values["extraVolumes"].([]corev1.Volume)[0]
	if assert.NotNil(t, extraVolume.HostPath) {
		assert.Equal(t, "/etc/ssl/certs/ca-certificates.crt", extraVolume.HostPath.Path)
		assert.Equal(t, corev1.HostPathFileOrCreate, *extraVolume.HostPath.Type)
	}

	extraVolumeMount := values["extraVolumeMounts"].([]corev1.VolumeMount)[0]
	assert.Equal(t, "host-ca-bundle", extraVolumeMount.Name)
	assert.Equal(t, "/certs/ca-certificates.crt", extraVolumeMount.MountPath)

	extraEnvVars := values["configuration"].(map[string]any)["extraEnvVars"].(map[string]any)
	assert.Equal(t, "/certs", extraEnvVars["SSL_CERT_DIR"])
}
