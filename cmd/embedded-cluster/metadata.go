package main

import (
	"encoding/json"
	"fmt"
	"sort"

	eckinds "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-kinds/types"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

var metadataCommand = &cli.Command{
	Name:   "metadata",
	Usage:  "Print metadata about this release",
	Hidden: true,
	Action: func(c *cli.Context) error {
		metadata, err := gatherVersionMetadata()
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
func gatherVersionMetadata() (*types.ReleaseMetadata, error) {
	applier := addons.NewApplier(
		addons.WithoutPrompt(),
		addons.OnlyDefaults(),
		addons.Quiet(),
	)

	versions, err := applier.Versions(config.AdditionalCharts())
	if err != nil {
		return nil, fmt.Errorf("unable to get versions: %w", err)
	}
	versions["Kubernetes"] = defaults.K0sVersion
	versions["Installer"] = defaults.Version
	versions["Troubleshoot"] = defaults.TroubleshootVersion
	versions["Kubectl"] = defaults.KubectlVersion

	channelRelease, err := release.GetChannelRelease()
	if err == nil && channelRelease != nil {
		versions[defaults.BinaryName()] = channelRelease.VersionLabel
	}

	sha, err := goods.K0sBinarySHA256()
	if err != nil {
		return nil, fmt.Errorf("unable to get k0s binary sha256: %w", err)
	}

	artifacts := map[string]string{
		"kots":                        fmt.Sprintf("kots-binaries/%s.tar.gz", adminconsole.KotsVersion),
		"operator":                    fmt.Sprintf("operator-binaries/%s.tar.gz", embeddedclusteroperator.Metadata.Version),
		"local-artifact-mirror-image": defaults.LocalArtifactMirrorImage,
	}

	meta := types.ReleaseMetadata{
		Versions:  versions,
		K0sSHA:    sha,
		Artifacts: artifacts,
	}

	chtconfig, repconfig, err := applier.GenerateHelmConfigs(
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
	builtinCharts, err := applier.GetBuiltinCharts()
	if err != nil {
		return nil, fmt.Errorf("unable to get builtin charts: %w", err)
	}
	meta.BuiltinConfigs = builtinCharts

	cfg := config.RenderK0sConfig()
	meta.K0sImages = config.ListK0sImages(cfg)

	additionalImages, err := applier.GetAdditionalImages()
	if err != nil {
		return nil, fmt.Errorf("unable to get additional images: %w", err)
	}
	meta.K0sImages = append(meta.K0sImages, additionalImages...)

	meta.K0sImages = helpers.UniqueStringSlice(meta.K0sImages)
	sort.Strings(meta.K0sImages)

	meta.Images = config.ListK0sImages(cfg)

	images, err := applier.GetImages()
	if err != nil {
		return nil, fmt.Errorf("unable to get images: %w", err)
	}
	meta.Images = append(meta.Images, images...)

	meta.Images = append(meta.Images, defaults.LocalArtifactMirrorImage)

	meta.Images = helpers.UniqueStringSlice(meta.Images)
	sort.Strings(meta.Images)

	return &meta, nil
}
