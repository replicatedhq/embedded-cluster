package template

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_VersionLabel(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	releaseData := &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{
			Spec: ecv1beta1.ConfigSpec{},
		},
		ChannelRelease: &release.ChannelRelease{
			VersionLabel: "v1.2.3",
		},
	}

	engine := NewEngine(config, WithReleaseData(releaseData))

	err := engine.Parse("{{repl VersionLabel }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", result)
}

func TestEngine_VersionLabelEmpty(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	releaseData := &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{
			Spec: ecv1beta1.ConfigSpec{},
		},
		ChannelRelease: &release.ChannelRelease{
			VersionLabel: "",
		},
	}

	engine := NewEngine(config, WithReleaseData(releaseData))

	err := engine.Parse("{{repl VersionLabel }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestEngine_VersionLabelWithoutReleaseData(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)

	err := engine.Parse("{{repl VersionLabel }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "release data is nil")
}

func TestEngine_VersionLabelWithoutChannelRelease(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	releaseData := &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{
			Spec: ecv1beta1.ConfigSpec{},
		},
		ChannelRelease: nil,
	}

	engine := NewEngine(config, WithReleaseData(releaseData))

	err := engine.Parse("{{repl VersionLabel }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel release is nil")
}

func TestEngine_VersionLabelInTemplate(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	releaseData := &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{
			Spec: ecv1beta1.ConfigSpec{},
		},
		ChannelRelease: &release.ChannelRelease{
			VersionLabel: "2024.1.0",
		},
	}

	engine := NewEngine(config, WithReleaseData(releaseData))

	err := engine.Parse("version: {{repl VersionLabel }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "version: 2024.1.0", result)
}
