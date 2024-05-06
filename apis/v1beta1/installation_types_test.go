package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestParseConfigSpecFromSecret(t *testing.T) {
	type test struct {
		Name       string            `yaml:"name"`
		SecretData map[string]string `yaml:"secret"`
		ConfigSpec *ConfigSpec       `yaml:"configSpec"`
		Expected   *ConfigSpec       `yaml:"expected"`
	}

	for tname, tt := range parseTestsYAML[test](t, "config-override-") {
		t.Run(tname, func(t *testing.T) {
			in := &InstallationSpec{Config: tt.ConfigSpec}
			secret := v1.Secret{
				Data: map[string][]byte{},
			}
			for k, v := range tt.SecretData {
				secret.Data[k] = []byte(v)
			}
			err := in.ParseConfigSpecFromSecret(secret)
			require.NoError(t, err)
			require.Equal(t, tt.Expected, in.Config)
		})
	}
}
