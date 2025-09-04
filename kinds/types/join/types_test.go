package join

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k0sproject/dig"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	k8syaml "sigs.k8s.io/yaml"
)

//go:embed testdata/*
var testData embed.FS

func TestJoinCommandResponseOverrides(t *testing.T) {
	type test struct {
		Name                      string
		EmbeddedOverrides         string `yaml:"embeddedOverrides"`
		EndUserOverrides          string `yaml:"endUserOverrides"`
		ExpectedEmbeddedOverrides string `yaml:"expectedEmbeddedOverrides"`
		ExpectedUserOverrides     string `yaml:"expectedUserOverrides"`
	}
	for tname, tt := range parseTestsYAML[test](t, "join-command-response-") {
		t.Run(tname, func(t *testing.T) {
			req := require.New(t)
			join := JoinCommandResponse{
				InstallationSpec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						UnsupportedOverrides: ecv1beta1.UnsupportedOverrides{
							K0s: tt.EmbeddedOverrides,
						},
					},
					EndUserK0sConfigOverrides: tt.EndUserOverrides,
				},
			}

			embedded, err := join.EmbeddedOverrides()
			req.NoError(err, "unable to patch config")
			expectedEmbedded := dig.Mapping{}
			err = yaml.Unmarshal([]byte(tt.ExpectedEmbeddedOverrides), &expectedEmbedded)
			req.NoError(err, "unable to unmarshal expected file")
			embeddedStr := fmt.Sprintf("%+v", embedded)
			expectedEmbeddedStr := fmt.Sprintf("%+v", expectedEmbedded)
			assert.Equal(t, expectedEmbeddedStr, embeddedStr)

			user, err := join.EndUserOverrides()
			req.NoError(err, "unable to patch config")
			expectedUser := dig.Mapping{}
			err = yaml.Unmarshal([]byte(tt.ExpectedUserOverrides), &expectedUser)
			req.NoError(err, "unable to unmarshal expected file")
			userStr := fmt.Sprintf("%+v", user)
			expectedUserStr := fmt.Sprintf("%+v", expectedUser)
			assert.Equal(t, expectedUserStr, userStr)
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
		err = k8syaml.Unmarshal(data, &onetest)
		require.NoError(t, err)

		tests[fpath] = onetest
	}
	return tests
}
