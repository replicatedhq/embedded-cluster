package config

import (
	"embed"
	"path/filepath"
	"strings"
	"testing"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
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
		Name     string
		Config   string `yaml:"config"`
		Override string `yaml:"override"`
		Expected string `yaml:"expected"`
	}

	for tname, tt := range parseTestsYAML[test](t, "override-") {
		t.Run(tname, func(t *testing.T) {
			req := require.New(t)

			var config k0sconfig.ClusterConfig
			err := k8syaml.Unmarshal([]byte(tt.Config), &config)
			req.NoError(err)

			result, err := PatchK0sConfig(&config, tt.Override)
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
		Name     string
		Override string `yaml:"override"`
		Expected string `yaml:"expected"`
	}

	for tname, tt := range parseTestsYAML[test](t, "extract-") {
		t.Run(tname, func(t *testing.T) {
			req := require.New(t)

			extracted, err := extractK0sConfigPatch(tt.Override)
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
