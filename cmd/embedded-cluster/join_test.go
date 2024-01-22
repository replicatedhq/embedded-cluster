package main

import (
	"embed"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/k0sproject/dig"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	k8syaml "sigs.k8s.io/yaml"
)

//go:embed testdata/*
var testData embed.FS

func Test_patchK0sConfig(t *testing.T) {
	type test struct {
		Name     string
		Original string `yaml:"original"`
		Override string `yaml:"override"`
		Expected string `yaml:"expected"`
	}
	entries, err := testData.ReadDir("testdata/patch-k0s-config")
	assert.NoError(t, err)
	var tests []test
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "skip.") {
			continue
		}
		fpath := path.Join("testdata", "patch-k0s-config", entry.Name())
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

			err = patchK0sConfig(originalFile.Name(), patch)
			req.NoError(err, "unable to patch config")

			var original dig.Mapping
			err = yaml.NewDecoder(originalFile).Decode(&original)
			req.NoError(err, "unable to decode original file")

			var expected dig.Mapping
			err = yaml.Unmarshal([]byte(tt.Expected), &expected)
			req.NoError(err, "unable to unmarshal expected file")

			assert.Equal(t, expected, original)
		})
	}
}

func TestJoinCommandResponseOverrides(t *testing.T) {
	type test struct {
		Name                      string
		EmbeddedOverrides         string `yaml:"embeddedOverrides"`
		EndUserOverrides          string `yaml:"endUserOverrides"`
		ExpectedEmbeddedOverrides string `yaml:"expectedEmbeddedOverrides"`
		ExpectedUserOverrides     string `yaml:"expectedUserOverrides"`
	}
	entries, err := testData.ReadDir("testdata/join-command-response")
	assert.NoError(t, err)
	var tests []test
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "skip.") {
			continue
		}
		fpath := path.Join("testdata", "join-command-response", entry.Name())
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
			join := JoinCommandResponse{
				K0sUnsupportedOverrides:   tt.EmbeddedOverrides,
				EndUserK0sConfigOverrides: tt.EndUserOverrides,
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
