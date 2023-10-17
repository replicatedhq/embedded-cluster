package config

import (
	"embed"
	"path"
	"strings"
	"testing"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

//go:embed testdata/*yaml
var testData embed.FS

func TestApplyUnsupportedOverrides(t *testing.T) {
	type test struct {
		Name     string
		Config   string `yaml:"config"`
		Override string `yaml:"override"`
		Expected string `yaml:"expected"`
	}
	entries, err := testData.ReadDir("testdata")
	assert.NoError(t, err)
	var tests []test
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "override") {
			continue
		}
		fpath := path.Join("testdata", entry.Name())
		data, err := testData.ReadFile(fpath)
		assert.NoError(t, err)
		var onetest test
		err = yaml.Unmarshal(data, &onetest)
		assert.NoError(t, err)
		onetest.Name = fpath
		tests = append(tests, onetest)
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			var config v1beta1.Cluster
			err := yaml.Unmarshal([]byte(tt.Config), &config)
			assert.NoError(t, err)
			err = ApplyEmbeddedUnsupportedOverrides(&config, []byte(tt.Override))
			assert.NoError(t, err)
			result, err := yaml.Marshal(config)
			assert.NoError(t, err)
			resultString := strings.TrimSpace(string(result))
			expectedString := strings.TrimSpace(string(tt.Expected))
			assert.Equal(t, resultString, expectedString)
		})
	}
}
