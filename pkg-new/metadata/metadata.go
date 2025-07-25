package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/gosimple/slug"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/extensions"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateVersionMetadataConfigmap(ctx context.Context, kcli client.Client) error {
	// This metadata should be the same as the artifact from the release without the vendor customizations
	metadata, err := GatherVersionMetadata(nil)
	if err != nil {
		return fmt.Errorf("gather release metadata: %w", err)
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal release metadata: %w", err)
	}

	// we trim out the prefix v from the version and then slugify it, we use
	// the result as a suffix for the config map name.
	slugver := slug.Make(strings.TrimPrefix(versions.Version, "v"))
	configmap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("version-metadata-%s", slugver),
			Namespace: "embedded-cluster",
			Labels: map[string]string{
				"replicated.com/disaster-recovery": "ec-install",
			},
		},
		Data: map[string]string{
			"metadata.json": string(data),
		},
	}

	if err := kcli.Create(ctx, configmap); err != nil {
		return fmt.Errorf("create version metadata config map: %w", err)
	}
	return nil
}

// GatherVersionMetadata returns the release metadata for this version of
// embedded cluster. Release metadata involves the default versions of the
// components that are included in the release plus the default values used
// when deploying them.
func GatherVersionMetadata(channelRelease *release.ChannelRelease) (*types.ReleaseMetadata, error) {
	versionsMap := map[string]string{}
	for name, version := range addons.Versions() {
		versionsMap[name] = version
	}
	if channelRelease != nil {
		for name, version := range extensions.Versions() {
			versionsMap[name] = version
		}
	}

	versionsMap["Kubernetes"] = versions.K0sVersion
	versionsMap["Installer"] = versions.Version
	versionsMap["Troubleshoot"] = versions.TroubleshootVersion

	if channelRelease != nil {
		versionsMap[runtimeconfig.AppSlug()] = channelRelease.VersionLabel
	}

	sha, err := goods.K0sBinarySHA256()
	if err != nil {
		return nil, fmt.Errorf("get k0s binary sha256: %w", err)
	}

	artifacts := map[string]string{
		"k0s":                         fmt.Sprintf("k0s-binaries/%s-%s", versions.K0sVersion, helpers.ClusterArch()),
		"kots":                        fmt.Sprintf("kots-binaries/%s-%s.tar.gz", adminconsole.KotsVersion, helpers.ClusterArch()),
		"operator":                    fmt.Sprintf("operator-binaries/%s-%s.tar.gz", embeddedclusteroperator.Metadata.Version, helpers.ClusterArch()),
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

	chtconfig, repconfig, err := addons.GenerateChartConfigs()
	if err != nil {
		return nil, fmt.Errorf("generate chart configs: %w", err)
	}

	additionalCharts := []ecv1beta1.Chart{}
	additionalRepos := []k0sv1beta1.Repository{}
	if channelRelease != nil {
		additionalCharts = config.AdditionalCharts()
		additionalRepos = config.AdditionalRepositories()
	}

	meta.Configs = ecv1beta1.Helm{
		ConcurrencyLevel: 1,
		Charts:           append(chtconfig, additionalCharts...),
		Repositories:     append(repconfig, additionalRepos...),
	}

	k0sCfg := config.RenderK0sConfig(domains.DefaultProxyRegistryDomain)
	meta.K0sImages = config.ListK0sImages(k0sCfg)
	meta.K0sImages = append(meta.K0sImages, addons.GetAdditionalImages()...)
	meta.K0sImages = helpers.UniqueStringSlice(meta.K0sImages)
	sort.Strings(meta.K0sImages)

	meta.Images = config.ListK0sImages(k0sCfg)
	meta.Images = append(meta.Images, addons.GetImages()...)
	meta.Images = append(meta.Images, versions.LocalArtifactMirrorImage)
	meta.Images = helpers.UniqueStringSlice(meta.Images)
	sort.Strings(meta.Images)

	return &meta, nil
}
