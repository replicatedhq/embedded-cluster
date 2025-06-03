package config

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg-new/paths"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	k8syaml "sigs.k8s.io/yaml"
)

//go:embed testdata/*
var testData embed.FS

func parseTestsYAML[T any](t *testing.T, prefix string) map[string]T {
	entries, err := testData.ReadDir("testdata")
	require.NoError(t, err)
	tests := make(map[string]T, 0)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}

		fpath := filepath.Join("testdata", entry.Name())
		data, err := testData.ReadFile(fpath)
		require.NoError(t, err)

		var onetest T
		err = yaml.Unmarshal(data, &onetest)
		require.NoError(t, err)

		tests[fpath] = onetest
	}
	return tests
}

func TestPatchK0sConfig(t *testing.T) {
	type test struct {
		Name                   string
		Config                 string `yaml:"config"`
		Override               string `yaml:"override"`
		Expected               string `yaml:"expected"`
		RespectImmutableFields bool   `yaml:"respectImmutableFields"`
	}

	for tname, tt := range parseTestsYAML[test](t, "override-") {
		t.Run(tname, func(t *testing.T) {
			req := require.New(t)

			var config k0sconfig.ClusterConfig
			err := k8syaml.Unmarshal([]byte(tt.Config), &config)
			req.NoError(err)

			result, err := PatchK0sConfig(&config, tt.Override, tt.RespectImmutableFields)
			req.NoError(err)

			var expected k0sconfig.ClusterConfig
			err = k8syaml.Unmarshal([]byte(tt.Expected), &expected)
			req.NoError(err)

			req.Equal(&expected, result)
		})
	}
}

func Test_extractK0sConfigPatch(t *testing.T) {
	type test struct {
		Name                   string
		Override               string `yaml:"override"`
		Expected               string `yaml:"expected"`
		RespectImmutableFields bool   `yaml:"respectImmutableFields"`
	}

	for tname, tt := range parseTestsYAML[test](t, "extract-") {
		t.Run(tname, func(t *testing.T) {
			req := require.New(t)

			extracted, err := extractK0sConfigPatch(tt.Override, tt.RespectImmutableFields)
			req.NoError(err)

			var actual map[string]interface{}
			err = k8syaml.Unmarshal([]byte(extracted), &actual)
			req.NoError(err)

			var expected map[string]interface{}
			err = k8syaml.Unmarshal([]byte(tt.Expected), &expected)
			req.NoError(err)

			req.Equal(expected, actual)
		})
	}
}

func TestRenderK0sConfig(t *testing.T) {
	cfg := RenderK0sConfig(domains.DefaultProxyRegistryDomain)

	assert.Equal(t, "calico", cfg.Spec.Network.Provider)
	assert.Equal(t, DefaultServiceNodePortRange, cfg.Spec.API.ExtraArgs["service-node-port-range"])
	assert.Contains(t, cfg.Spec.API.SANs, "kubernetes.default.svc.cluster.local")
	val, err := json.Marshal(&cfg.Spec.Telemetry.Enabled)
	require.NoError(t, err)
	assert.Equal(t, "false", string(val))
}

func TestInstallFlags(t *testing.T) {
	// Create a pair of temporary k0s config files
	k0sCfg := k0sconfig.DefaultClusterConfig()
	k0sDefaultConfigBytes, err := k8syaml.Marshal(k0sCfg)
	require.NoError(t, err)

	defaultTmpFile, err := os.CreateTemp("", "k0s-*.yaml")
	require.NoError(t, err)
	defer os.Remove(defaultTmpFile.Name())

	err = os.WriteFile(defaultTmpFile.Name(), k0sDefaultConfigBytes, 0644)
	require.NoError(t, err)

	k0sCfg.Spec.WorkerProfiles = []k0sconfig.WorkerProfile{
		{
			Name: "test-profile",
		},
	}
	k0sProfileConfigBytes, err := k8syaml.Marshal(k0sCfg)
	require.NoError(t, err)

	profileTmpFile, err := os.CreateTemp("", "k0s-*.yaml")
	require.NoError(t, err)
	defer os.Remove(profileTmpFile.Name())

	err = os.WriteFile(profileTmpFile.Name(), k0sProfileConfigBytes, 0644)
	require.NoError(t, err)

	tests := []struct {
		name           string
		nodeIP         string
		releaseData    map[string][]byte
		expectedFlags  []string
		expectedError  bool
		expectedErrMsg string
		k0sConfigPath  string
	}{
		{
			name:          "default configuration",
			nodeIP:        "192.168.1.10",
			k0sConfigPath: defaultTmpFile.Name(),
			releaseData:   map[string][]byte{},
			expectedFlags: []string{
				"install",
				"controller",
				"--labels", "kots.io/embedded-cluster-role-0=controller,kots.io/embedded-cluster-role=total-1",
				"--enable-worker",
				"--no-taints",
				"-c", paths.PathToK0sConfig(),
				"--kubelet-extra-args", "--node-ip=192.168.1.10",
				"--data-dir", runtimeconfig.EmbeddedClusterK0sSubDir(),
				"--disable-components", "konnectivity-server",
				"--enable-dynamic-config",
			},
			expectedError: false,
		},
		{
			name:          "custom controller role name with worker profile",
			nodeIP:        "192.168.1.10",
			k0sConfigPath: profileTmpFile.Name(),
			releaseData: map[string][]byte{
				"cluster-config.yaml": []byte(`
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
metadata:
  name: embedded-cluster
spec:
  roles:
    controller:
      name: custom-controller
      labels:
        environment: test
`),
			},
			expectedFlags: []string{
				"install",
				"controller",
				"--labels", "environment=test,kots.io/embedded-cluster-role-0=custom-controller,kots.io/embedded-cluster-role=total-1",
				"--enable-worker",
				"--no-taints",
				"-c", paths.PathToK0sConfig(),
				"--profile=test-profile",
				"--kubelet-extra-args", "--node-ip=192.168.1.10",
				"--data-dir", runtimeconfig.EmbeddedClusterK0sSubDir(),
				"--disable-components", "konnectivity-server",
				"--enable-dynamic-config",
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test data
			err := release.SetReleaseDataForTests(tt.releaseData)
			require.NoError(t, err)

			// Set the override for the k0s config path
			k0sConfigPathOverride = tt.k0sConfigPath

			// Cleanup after test
			t.Cleanup(func() {
				release.SetReleaseDataForTests(nil)
				k0sConfigPathOverride = ""
			})

			// Run test
			flags, err := InstallFlags(tt.nodeIP)
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedFlags, flags)
			}
		})
	}
}
