package velero

import (
	"context"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateHelmValues_HostCABundlePath(t *testing.T) {
	v := &Velero{
		HostCABundlePath: "/etc/ssl/certs/ca-certificates.crt",
	}

	values, err := v.GenerateHelmValues(context.Background(), nil, ecv1beta1.Domains{}, nil)
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

func TestGenerateHelmValues_NoPlugins(t *testing.T) {
	v := &Velero{
		EmbeddedConfigSpec: nil,
	}

	values, err := v.GenerateHelmValues(context.Background(), nil, ecv1beta1.Domains{}, nil)
	require.NoError(t, err)

	// Should have at most the default AWS plugin
	initContainers, ok := values["initContainers"].([]any)
	require.True(t, ok, "initContainers should be a slice")
	assert.LessOrEqual(t, len(initContainers), 1, "Should have at most the default AWS plugin")
}

func TestGenerateHelmValues_SinglePlugin(t *testing.T) {
	v := &Velero{
		EmbeddedConfigSpec: &ecv1beta1.ConfigSpec{
			Extensions: ecv1beta1.Extensions{
				Velero: ecv1beta1.VeleroExtensions{
					Plugins: []ecv1beta1.VeleroPlugin{
						{
							Name:  "velero-plugin-postgresql",
							Image: "myvendor/velero-postgresql:v1.0.0",
						},
					},
				},
			},
		},
	}

	values, err := v.GenerateHelmValues(context.Background(), nil, ecv1beta1.Domains{}, nil)
	require.NoError(t, err)

	require.NotEmpty(t, values["initContainers"])
	initContainers, ok := values["initContainers"].([]any)
	require.True(t, ok, "initContainers should be a slice")
	assert.Equal(t, len(initContainers), 2, "Should have exactly 2 init containers: velero-plugin-postgresql and AWS plugin")

	// Check that we have the custom plugin with volumeMounts
	var foundPostgresPlugin bool
	var foundAWSPlugin bool
	for _, container := range initContainers {
		containerMap, ok := container.(map[string]any)
		require.True(t, ok, "container should be a map")

		if name, _ := containerMap["name"].(string); name == "velero-plugin-postgresql" {
			foundPostgresPlugin = true
			assert.Equal(t, "myvendor/velero-postgresql:v1.0.0", containerMap["image"])
			assert.Equal(t, "IfNotPresent", containerMap["imagePullPolicy"])
			assert.Contains(t, containerMap, "volumeMounts")
		} else if name == "velero-plugin-for-aws" {
			foundAWSPlugin = true
			assert.Equal(t, "IfNotPresent", containerMap["imagePullPolicy"])
			assert.Contains(t, containerMap, "volumeMounts")
		}
	}

	assert.True(t, foundPostgresPlugin, "Should have velero-plugin-postgresql")
	assert.True(t, foundAWSPlugin, "Should have velero-plugin-for-aws")
}

func TestGenerateHelmValues_MultiplePlugins(t *testing.T) {
	v := &Velero{
		EmbeddedConfigSpec: &ecv1beta1.ConfigSpec{
			Extensions: ecv1beta1.Extensions{
				Velero: ecv1beta1.VeleroExtensions{
					Plugins: []ecv1beta1.VeleroPlugin{
						{
							Name:  "velero-plugin-postgresql",
							Image: "myvendor/velero-postgresql:v1.0.0",
						},
						{
							Name:  "velero-plugin-mongodb",
							Image: "myvendor/velero-mongodb:v2.1.0",
						},
					},
				},
			},
		},
	}

	values, err := v.GenerateHelmValues(context.Background(), nil, ecv1beta1.Domains{}, nil)
	require.NoError(t, err)

	require.NotEmpty(t, values["initContainers"])
	initContainers, ok := values["initContainers"].([]any)
	require.True(t, ok, "initContainers should be a slice")
	assert.Equal(t, len(initContainers), 3, "Should have exactly 3 init containers: AWS, velero-plugin-postgresql, and velero-plugin-mongodb")

	// Check that we have the custom plugins with volumeMounts
	var foundPostgresPlugin bool
	var foundMongoDBPlugin bool
	var foundAWSPlugin bool
	for _, container := range initContainers {
		containerMap, ok := container.(map[string]any)
		require.True(t, ok, "container should be a map")

		name, _ := containerMap["name"].(string)
		switch name {
		case "velero-plugin-postgresql":
			foundPostgresPlugin = true
			assert.Equal(t, "myvendor/velero-postgresql:v1.0.0", containerMap["image"])
			assert.Equal(t, "IfNotPresent", containerMap["imagePullPolicy"])
			assert.Contains(t, containerMap, "volumeMounts")
		case "velero-plugin-mongodb":
			foundMongoDBPlugin = true
			assert.Equal(t, "myvendor/velero-mongodb:v2.1.0", containerMap["image"])
			assert.Equal(t, "IfNotPresent", containerMap["imagePullPolicy"])
			assert.Contains(t, containerMap, "volumeMounts")
		case "velero-plugin-for-aws":
			foundAWSPlugin = true
			assert.Equal(t, "IfNotPresent", containerMap["imagePullPolicy"])
			assert.Contains(t, containerMap, "volumeMounts")
		}
	}

	assert.True(t, foundPostgresPlugin, "Should have velero-plugin-postgresql")
	assert.True(t, foundMongoDBPlugin, "Should have velero-plugin-mongodb")
	assert.True(t, foundAWSPlugin, "Should have velero-plugin-for-aws")
}

func TestGenerateHelmValues_PluginWithImagePullPolicy(t *testing.T) {
	v := &Velero{
		EmbeddedConfigSpec: &ecv1beta1.ConfigSpec{
			Extensions: ecv1beta1.Extensions{
				Velero: ecv1beta1.VeleroExtensions{
					Plugins: []ecv1beta1.VeleroPlugin{
						{
							Name:            "velero-plugin-postgresql",
							Image:           "myvendor/velero-postgresql:v1.0.0",
							ImagePullPolicy: "Always",
						},
					},
				},
			},
		},
	}

	values, err := v.GenerateHelmValues(context.Background(), nil, ecv1beta1.Domains{}, nil)
	require.NoError(t, err)

	require.NotEmpty(t, values["initContainers"])
	initContainers, ok := values["initContainers"].([]any)
	require.True(t, ok, "initContainers should be a slice")
	assert.Equal(t, len(initContainers), 2, "Should have exactly 2 init containers: velero-plugin-postgresql and AWS plugin")

	// Check that we have the custom plugin with the custom imagePullPolicy
	var foundPostgresPlugin bool
	for _, container := range initContainers {
		containerMap, ok := container.(map[string]any)
		require.True(t, ok, "container should be a map")

		if name, _ := containerMap["name"].(string); name == "velero-plugin-postgresql" {
			foundPostgresPlugin = true
			assert.Equal(t, "myvendor/velero-postgresql:v1.0.0", containerMap["image"])
			assert.Equal(t, "Always", containerMap["imagePullPolicy"])
			assert.Contains(t, containerMap, "volumeMounts")
		}
	}

	assert.True(t, foundPostgresPlugin, "Should have velero-plugin-postgresql with ImagePullPolicy=Always")
}

func TestGenerateHelmValues_PluginVolumeMounts(t *testing.T) {
	v := &Velero{
		EmbeddedConfigSpec: &ecv1beta1.ConfigSpec{
			Extensions: ecv1beta1.Extensions{
				Velero: ecv1beta1.VeleroExtensions{
					Plugins: []ecv1beta1.VeleroPlugin{
						{
							Name:  "velero-plugin-postgresql",
							Image: "myvendor/velero-postgresql:v1.0.0",
						},
					},
				},
			},
		},
	}

	values, err := v.GenerateHelmValues(context.Background(), nil, ecv1beta1.Domains{}, nil)
	require.NoError(t, err)

	require.NotEmpty(t, values["initContainers"])
	initContainers, ok := values["initContainers"].([]any)
	require.True(t, ok, "initContainers should be a slice")
	assert.Equal(t, len(initContainers), 2, "Should have exactly 2 init containers, velero-plugin-postgresql and AWS")

	// Find our plugin container
	var pluginContainer map[string]any
	for _, container := range initContainers {
		if containerMap, ok := container.(map[string]any); ok {
			if name, _ := containerMap["name"].(string); name == "velero-plugin-postgresql" {
				pluginContainer = containerMap
				break
			}
		}
	}

	require.NotNil(t, pluginContainer)
	require.Contains(t, pluginContainer, "volumeMounts")

	volumeMounts, ok := pluginContainer["volumeMounts"].([]any)
	require.True(t, ok, "volumeMounts should be []any")

	require.Len(t, volumeMounts, 1)
	volumeMount, ok := volumeMounts[0].(map[string]any)
	require.True(t, ok, "volumeMount should be map[string]any")
	assert.Equal(t, "/target", volumeMount["mountPath"])
	assert.Equal(t, "plugins", volumeMount["name"])
}

func TestGenerateHelmValues_CustomDomainReplacement(t *testing.T) {
	tests := []struct {
		name                string
		proxyRegistryDomain string
		expectReplacement   bool
		expectedDomain      string
	}{
		{
			name:                "empty domain should not replace",
			proxyRegistryDomain: "",
			expectReplacement:   false,
		},
		{
			name:                "custom domain should replace",
			proxyRegistryDomain: "custom-registry.example.com",
			expectReplacement:   true,
			expectedDomain:      "custom-registry.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Velero{}
			domains := ecv1beta1.Domains{
				ProxyRegistryDomain: tt.proxyRegistryDomain,
			}

			values, err := v.GenerateHelmValues(context.Background(), nil, domains, nil)
			require.NoError(t, err)

			// Marshal values back to YAML to check for domain replacement
			marshalled, err := helm.MarshalValues(values)
			require.NoError(t, err)
			marshalledStr := string(marshalled)

			if tt.expectReplacement {
				assert.NotContains(t, marshalledStr, "proxy.replicated.com", "should not contain proxy.replicated.com")
				assert.Contains(t, marshalledStr, tt.expectedDomain, "should contain custom domain")
			} else {
				assert.Contains(t, marshalledStr, "proxy.replicated.com", "should contain proxy.replicated.com when domain is empty")
			}
		})
	}
}
