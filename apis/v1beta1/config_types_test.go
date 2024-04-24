package v1beta1

import (
	"embed"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
		err = k8syaml.Unmarshal(data, &onetest)
		require.NoError(t, err)

		tests[fpath] = onetest
	}
	return tests
}

func TestApplyEndUserAddOnOverrides(t *testing.T) {
	type config struct {
		Name   string `yaml:"name"`
		Values string `yaml:"values"`
	}

	type test struct {
		Name     string     `yaml:"name"`
		Spec     ConfigSpec `yaml:"spec"`
		Config   config     `yaml:"config"`
		Expected string     `yaml:"expected"`
	}

	for tname, tt := range parseTestsYAML[test](t, "addon-override-") {
		t.Run(tname, func(t *testing.T) {
			raw, err := tt.Spec.ApplyEndUserAddOnOverrides(tt.Config.Name, tt.Config.Values)
			require.NoError(t, err)

			result := make(map[string]interface{})
			err = k8syaml.Unmarshal([]byte(raw), &result)
			require.NoError(t, err)

			expected := make(map[string]interface{})
			err = k8syaml.Unmarshal([]byte(tt.Expected), &expected)
			require.NoError(t, err)

			require.Equal(t, expected, result)
		})
	}
}
