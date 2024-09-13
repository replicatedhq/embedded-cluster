package main

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sort"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	eckinds "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
)

var metadataCommand = &cli.Command{
	Name:   "metadata",
	Usage:  "Print metadata about this release",
	Hidden: true,
	Action: func(c *cli.Context) error {
		k0sCfg := config.RenderK0sConfig()
		metadata, err := gatherVersionMetadata(k0sCfg)
		if err != nil {
			return fmt.Errorf("failed to gather version metadata: %w", err)
		}
		data, err := json.MarshalIndent(metadata, "", "\t")
		if err != nil {
			return fmt.Errorf("failed to marshal versions: %w", err)
		}
		fmt.Println(string(data))
		return nil
	},
}

// gatherVersionMetadata returns the release metadata for this version of
// embedded cluster. Release metadata involves the default versions of the
// components that are included in the release plus the default values used
// when deploying them.
func gatherVersionMetadata(k0sCfg *k0sconfig.ClusterConfig) (*types.ReleaseMetadata, error) {
	applier := addons.NewApplier(
		addons.WithoutPrompt(),
		addons.OnlyDefaults(),
		addons.Quiet(),
	)

	versionsMap, err := applier.Versions(config.AdditionalCharts())
	if err != nil {
		return nil, fmt.Errorf("unable to get versions: %w", err)
	}
	versionsMap["Kubernetes"] = versions.K0sVersion
	versionsMap["Installer"] = versions.Version
	versionsMap["Troubleshoot"] = versions.TroubleshootVersion

	channelRelease, err := release.GetChannelRelease()
	if err == nil && channelRelease != nil {
		versionsMap[defaults.BinaryName()] = channelRelease.VersionLabel
	}

	sha, err := goods.K0sBinarySHA256()
	if err != nil {
		return nil, fmt.Errorf("unable to get k0s binary sha256: %w", err)
	}

	artifacts := map[string]string{
		"k0s":                         fmt.Sprintf("k0s-binaries/%s-%s", versions.K0sVersion, runtime.GOARCH),
		"kots":                        fmt.Sprintf("kots-binaries/%s-%s.tar.gz", adminconsole.KotsVersion, runtime.GOARCH),
		"operator":                    fmt.Sprintf("operator-binaries/%s-%s.tar.gz", embeddedclusteroperator.Metadata.Version, runtime.GOARCH),
		"local-artifact-mirror-image": versions.LocalArtifactMirrorImage,
	}
	if versions.K0sBinaryURLOverride != "" {
		artifacts["k0s"] = versions.K0sBinaryURLOverride
	}
	if versions.KOTSBinaryURLOverride != "" {
		artifacts["kots"] = versions.KOTSBinaryURLOverride
	}
	if versions.OperatorBinaryURLOverride != "" {
		artifacts["operator"] = versions.OperatorBinaryURLOverride
	}

	meta := types.ReleaseMetadata{
		Versions:  versionsMap,
		K0sSHA:    sha,
		Artifacts: artifacts,
	}

	chtconfig, repconfig, err := applier.GenerateHelmConfigs(
		k0sCfg,
		config.AdditionalCharts(),
		config.AdditionalRepositories(),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to apply addons: %w", err)
	}

	meta.Configs = eckinds.Helm{
		ConcurrencyLevel: 1,
		Charts:           chtconfig,
		Repositories:     repconfig,
	}

	protectedFields, err := applier.ProtectedFields()
	if err != nil {
		return nil, fmt.Errorf("unable to get protected fields: %w", err)
	}
	meta.Protected = protectedFields

	// Additional builtin addons
	builtinCharts, err := applier.GetBuiltinCharts(k0sCfg)
	if err != nil {
		return nil, fmt.Errorf("unable to get builtin charts: %w", err)
	}
	meta.BuiltinConfigs = builtinCharts

	meta.K0sImages = config.ListK0sImages(k0sCfg)

	additionalImages, err := applier.GetAdditionalImages()
	if err != nil {
		return nil, fmt.Errorf("unable to get additional images: %w", err)
	}
	meta.K0sImages = append(meta.K0sImages, additionalImages...)

	meta.K0sImages = helpers.UniqueStringSlice(meta.K0sImages)
	sort.Strings(meta.K0sImages)

	meta.Images = config.ListK0sImages(k0sCfg)

	images, err := applier.GetImages()
	if err != nil {
		return nil, fmt.Errorf("unable to get images: %w", err)
	}
	meta.Images = append(meta.Images, images...)

	meta.Images = append(meta.Images, versions.LocalArtifactMirrorImage)

	meta.Images = helpers.UniqueStringSlice(meta.Images)
	sort.Strings(meta.Images)

	return &meta, nil
}
