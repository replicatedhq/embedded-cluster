package addons

import (
	"embed"
	"path/filepath"
	"strings"
	"testing"

	"github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
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

func TestGenerateHelmConfigWithOverrides(t *testing.T) {
	type addonConfig struct {
		Name   string `yaml:"name"`
		Values string `yaml:"values"`
	}

	type test struct {
		Name          string
		EndUserConfig string `yaml:"endUserConfig"`
		Expected      []addonConfig
	}

	for tname, tt := range parseTestsYAML[test](t, "generate-helm-config-overrides-") {
		t.Run(tname, func(t *testing.T) {
			var config v1beta1.Config
			err := k8syaml.Unmarshal([]byte(tt.EndUserConfig), &config)
			require.NoError(t, err)
			applier := NewApplier(
				WithEndUserConfig(&config),
				WithoutPrompt(),
				WithAirgapBundle("/does/not/exist"),
			)
			charts, _, err := applier.GenerateHelmConfigs(nil, nil)
			require.NoError(t, err)

			for _, exp := range tt.Expected {
				var values string
				for _, chart := range charts {
					if chart.Name != exp.Name {
						continue
					}
					values = chart.Values
					break
				}
				require.NotEmpty(t, values, "addon %s not found", exp.Name)

				expected := map[string]interface{}{}
				err = yaml.Unmarshal([]byte(exp.Values), &expected)
				require.NoError(t, err)

				found := map[string]interface{}{}
				err = yaml.Unmarshal([]byte(values), &found)
				require.NoError(t, err)

				require.Equal(t, expected, found)
			}
		})
	}
}
