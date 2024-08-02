package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var k0sImageComponents = map[string]string{
	"quay.io/k0sproject/coredns":                    "coredns",
	"quay.io/k0sproject/calico-node":                "calico-node",
	"quay.io/k0sproject/calico-cni":                 "calico-cni",
	"quay.io/k0sproject/calico-kube-controllers":    "calico-kube-controllers",
	"registry.k8s.io/metrics-server/metrics-server": "metrics-server",
	"quay.io/k0sproject/kube-proxy":                 "kube-proxy",
	"quay.io/k0sproject/envoy-distroless":           "envoy-distroless",
	"registry.k8s.io/pause":                         "pause",
}

var k0sComponents = map[string]addonComponent{
	"coredns": {
		getImageName: func(opts addonComponentOptions) (string, error) {
			tag, err := GetGitHubRelease(opts.ctx, "coredns", "coredns", latestMinorTagFilter(opts.upstreamVersion))
			if err != nil {
				return "", fmt.Errorf("failed to get gh release: %w", err)
			}
			return fmt.Sprintf("coredns/coredns:%s", strings.TrimPrefix(tag, "v")), nil
		},
	},
	"calico-node": {
		getImageName: func(opts addonComponentOptions) (string, error) {
			tag, err := GetGitHubRelease(opts.ctx, "projectcalico", "calico", latestMinorTagFilter(opts.upstreamVersion))
			if err != nil {
				return "", fmt.Errorf("failed to get gh release: %w", err)
			}
			return fmt.Sprintf("calico/node:%s", tag), nil
		},
	},
	"calico-cni": {
		getImageName: func(opts addonComponentOptions) (string, error) {
			tag, err := GetGitHubRelease(opts.ctx, "projectcalico", "calico", latestMinorTagFilter(opts.upstreamVersion))
			if err != nil {
				return "", fmt.Errorf("failed to get gh release: %w", err)
			}
			return fmt.Sprintf("calico/cni:%s", tag), nil
		},
	},
	"calico-kube-controllers": {
		getImageName: func(opts addonComponentOptions) (string, error) {
			tag, err := GetGitHubRelease(opts.ctx, "projectcalico", "calico", latestMinorTagFilter(opts.upstreamVersion))
			if err != nil {
				return "", fmt.Errorf("failed to get gh release: %w", err)
			}
			return fmt.Sprintf("calico/kube-controllers:%s", tag), nil
		},
	},
	"metrics-server": {
		getImageName: func(opts addonComponentOptions) (string, error) {
			tag, err := GetGitHubRelease(opts.ctx, "kubernetes-sigs", "metrics-server", latestMinorTagFilter(opts.upstreamVersion))
			if err != nil {
				return "", fmt.Errorf("failed to get gh release: %w", err)
			}
			return fmt.Sprintf("registry.k8s.io/metrics-server/metrics-server:%s", tag), nil
		},
	},
	"kube-proxy": {
		getImageName: func(opts addonComponentOptions) (string, error) {
			tag, err := GetGitHubRelease(opts.ctx, "kubernetes", "kubernetes", latestPatchTagFilter(opts.upstreamVersion))
			if err != nil {
				return "", fmt.Errorf("failed to get gh release: %w", err)
			}
			return fmt.Sprintf("registry.k8s.io/kube-proxy:%s", tag), nil
		},
	},
	"envoy-distroless": {
		getImageName: func(opts addonComponentOptions) (string, error) {
			tag, err := GetGitHubRelease(opts.ctx, "envoyproxy", "envoy", latestMinorTagFilter(opts.upstreamVersion))
			if err != nil {
				return "", fmt.Errorf("failed to get gh release: %w", err)
			}
			return fmt.Sprintf("envoyproxy/envoy:%s", tag), nil
		},
	},
	"pause": {
		getImageName: func(opts addonComponentOptions) (string, error) {
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
			Images: make(map[string]release.K0sImage),
		}

		k0sImages := config.ListK0sImages(k0sconfig.DefaultClusterConfig())

		if err := ApkoLogin(); err != nil {
			return fmt.Errorf("failed to apko login: %w", err)
		}

		for _, image := range k0sImages {
			logrus.Infof("updating image %s", image)

			componentName, ok := k0sImageComponents[RemoveTagFromImage(image)]
			if !ok {
				return fmt.Errorf("no component found for image %s", image)
			}

			component, ok := k0sComponents[componentName]
			if !ok {
				return fmt.Errorf("no component found for component name %s", componentName)
			}

			k0sVersion, err := getK0sVersion()
			if err != nil {
				return fmt.Errorf("get k0s version: %w", err)
			}

			latestK8sVersion, err := GetLatestKubernetesVersion()
			if err != nil {
				return fmt.Errorf("get latest k8s version: %w", err)
			}

			upstreamVersion := TagFromImage(image)
			upstreamVersion = strings.TrimPrefix(upstreamVersion, "v")
			upstreamVersion = strings.Split(upstreamVersion, "-")[0]

			image, err = component.getImageName(addonComponentOptions{
				ctx:              c.Context,
				k0sVersion:       k0sVersion,
				upstreamVersion:  semver.MustParse(upstreamVersion),
				latestK8sVersion: latestK8sVersion,
			})
			if err != nil {
				return fmt.Errorf("failed to get image name for %s: %w", componentName, err)
			}

			logrus.Infof("fetching digest for image %s", image)
			sha, err := GetImageDigest(c.Context, image)
			if err != nil {
				return fmt.Errorf("failed to get image %s digest: %w", image, err)
			}
			logrus.Infof("image %s digest: %s", image, sha)

			newmeta.Images[componentName] = release.K0sImage{
				Image:   fmt.Sprintf("proxy.replicated.com/anonymous/%s", FamiliarImageName(image)),
				Version: fmt.Sprintf("%s@%s", TagFromImage(image), sha),
			}
		}

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
