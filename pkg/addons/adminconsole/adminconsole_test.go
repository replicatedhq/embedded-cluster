package adminconsole

import (
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGenerateHelmConfigPrivateCAs(t *testing.T) {
	cas := map[string]string{
		"ca_0.crt": "ca1-content",
		"ca_1.crt": "ca2-content",
	}

	ac, err := New("", "", "license.yaml", "", nil, cas)
	assert.NoError(t, err)

	var kcfg v1beta1.ClusterConfig
	cfgs, repo, err := ac.GenerateHelmConfig(&kcfg, true)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 1)
	assert.Len(t, repo, 0)

	cfg := cfgs[0]
	var values map[string]interface{}
	err = yaml.Unmarshal([]byte(cfg.Values), &values)
	assert.NoError(t, err)
	require.NotContains(t, values, "privateCAs")

	cfgs, repo, err = ac.GenerateHelmConfig(&kcfg, false)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 1)
	assert.Len(t, repo, 0)

	cfg = cfgs[0]
	err = yaml.Unmarshal([]byte(cfg.Values), &values)
	assert.NoError(t, err)
	require.Contains(t, values, "privateCAs")

	type privateCAs struct {
		PrivateCAs map[string]string `yaml:"privateCAs"`
	}
	var private privateCAs
	err = yaml.Unmarshal([]byte(cfg.Values), &private)
	assert.NoError(t, err)
	assert.Equal(t, cas, private.PrivateCAs)
}
