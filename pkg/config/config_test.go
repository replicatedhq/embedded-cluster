package config

import (
	"embed"
	"path"
	"strings"
	"testing"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	kyaml "sigs.k8s.io/yaml"
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
			req := require.New(t)

			var config v1beta1.Cluster
			err := yaml.Unmarshal([]byte(tt.Config), &config)
			req.NoError(err)
			var cfg embeddedclusterv1beta1.Config
			err = kyaml.Unmarshal([]byte(tt.Override), &cfg)
			req.NoError(err)
			err = ApplyEmbeddedUnsupportedOverrides(&config, cfg.Spec.UnsupportedOverrides.K0s)
			req.NoError(err)
			result, err := yaml.Marshal(config)
			req.NoError(err)
			resultString := strings.TrimSpace(string(result))
			expectedString := strings.TrimSpace(string(tt.Expected))
			req.Equal(expectedString, resultString)
		})
	}
}
