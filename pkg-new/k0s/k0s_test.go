package k0s

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	k8syaml "sigs.k8s.io/yaml"
)

//go:embed testdata/*
var testData embed.FS

func TestPatchK0sConfig(t *testing.T) {
	type test struct {
		Name     string
		Original string `yaml:"original"`
		Override string `yaml:"override"`
		Expected string `yaml:"expected"`
	}
	for tname, tt := range parseTestsYAML[test](t, "patch-k0s-config-") {
		t.Run(tname, func(t *testing.T) {
			req := require.New(t)

			originalFile, err := os.CreateTemp("", "k0s-original-*.yaml")
			req.NoError(err, "unable to create temp file")
			defer func() {
				originalFile.Close()
				os.Remove(originalFile.Name())
			}()
			err = os.WriteFile(originalFile.Name(), []byte(tt.Original), 0644)
			req.NoError(err, "unable to write original config")

			var patch string
			if tt.Override != "" {
				var overrides embeddedclusterv1beta1.Config
				err = k8syaml.Unmarshal([]byte(tt.Override), &overrides)
				req.NoError(err, "unable to unmarshal override")
				patch = overrides.Spec.UnsupportedOverrides.K0s
			}

			err = PatchK0sConfig(originalFile.Name(), patch)
			req.NoError(err, "unable to patch config")

			data, err := os.ReadFile(originalFile.Name())
			req.NoError(err, "unable to read patched config")

			var original k0sv1beta1.ClusterConfig
			err = k8syaml.Unmarshal(data, &original)
			req.NoError(err, "unable to decode original file")

			var expected k0sv1beta1.ClusterConfig
			err = k8syaml.Unmarshal([]byte(tt.Expected), &expected)
			req.NoError(err, "unable to unmarshal expected file")

			assert.Equal(t, expected, original)
		})
	}
}

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
