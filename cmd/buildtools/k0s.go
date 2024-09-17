package main

import (
	"fmt"
	"os"

	"github.com/Masterminds/semver/v3"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var k0sImageComponents = map[string]addonComponent{
	"quay.io/k0sproject/coredns": {
		name: "coredns",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "coredns"
		},
	},
	"quay.io/k0sproject/calico-node": {
		name: "calico-node",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "calico-node"
		},
	},
	"quay.io/k0sproject/calico-cni": {
		name: "calico-cni",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "calico-cni"
		},
	},
	"quay.io/k0sproject/calico-kube-controllers": {
		name: "calico-kube-controllers",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "calico-kube-controllers"
		},
	},
	"registry.k8s.io/metrics-server/metrics-server": {
		name: "metrics-server",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "metrics-server"
		},
	},
	"quay.io/k0sproject/metrics-server": {
		name: "metrics-server",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "metrics-server"
		},
	},
	"quay.io/k0sproject/kube-proxy": {
		name: "kube-proxy",
		getCustomImageName: func(opts addonComponentOptions) (string, error) {
			// latest patch version of the current minor version
			constraints := mustParseSemverConstraints(latestPatchConstraint(opts.upstreamVersion))
			tag, err := GetGreatestGitHubTag(opts.ctx, "kubernetes", "kubernetes", constraints)
			if err != nil {
				return "", fmt.Errorf("failed to get gh release: %w", err)
			}
			return fmt.Sprintf("registry.k8s.io/kube-proxy:%s", tag), nil
		},
	},
	"registry.k8s.io/pause": {
		name: "pause",
		getCustomImageName: func(opts addonComponentOptions) (string, error) {
			return fmt.Sprintf("registry.k8s.io/pause:%s", opts.upstreamVersion.Original()), nil
		},
	},
}

var updateK0sImagesCommand = &cli.Command{
	Name:      "k0s",
	Usage:     "Updates the k0s images",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating k0s images")

		newmeta := release.K0sMetadata{
			Images: make(map[string]release.AddonImage),
		}

		k0sImages := config.ListK0sImages(k0sv1beta1.DefaultClusterConfig())

		metaImages, err := UpdateImages(c.Context, k0sImageComponents, config.Metadata.Images, k0sImages)
		if err != nil {
			return fmt.Errorf("failed to update images: %w", err)
		}
		newmeta.Images = metaImages

		logrus.Infof("saving k0s metadata")
		if err := newmeta.Save(); err != nil {
			return fmt.Errorf("failed to save k0s metadata: %w", err)
		}

		return nil
	},
}

func getK0sVersion() (*semver.Version, error) {
	if v := os.Getenv("INPUT_K0S_VERSION"); v != "" {
		logrus.Infof("using input override from INPUT_K0S_VERSION: %s", v)
		return semver.MustParse(v), nil
	}
	v, err := GetMakefileVariable("K0S_VERSION")
	if err != nil {
		return nil, fmt.Errorf("failed to get k0s version: %w", err)
	}
	return semver.MustParse(v), nil
}
