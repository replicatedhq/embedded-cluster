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
		getWolfiPackageName: func(opts commonOptions) string {
			return "coredns"
		},
	},
	"calico-node": {
		getWolfiPackageName: func(opts commonOptions) string {
			return "calico-node"
		},
	},
	"calico-cni": {
		getWolfiPackageName: func(opts commonOptions) string {
			return "calico-cni"
		},
	},
	"calico-kube-controllers": {
		getWolfiPackageName: func(opts commonOptions) string {
			return "calico-kube-controllers"
		},
	},
	"metrics-server": {
		getWolfiPackageName: func(opts commonOptions) string {
			return "metrics-server"
		},
	},
	"kube-proxy": {
		getWolfiPackageName: func(opts commonOptions) string {
			return fmt.Sprintf("kube-proxy-%d.%d-default", opts.upstreamVersion.Major(), opts.upstreamVersion.Minor())
		},
	},
	"envoy-distroless": {
		getWolfiPackageName: func(opts commonOptions) string {
			return fmt.Sprintf("envoy-%d.%d", opts.upstreamVersion.Major(), opts.upstreamVersion.Minor())
		},
	},
	"pause": {
		getWolfiPackageName: func(opts commonOptions) string {
			return fmt.Sprintf("kubernetes-pause-%d.%d", opts.upstreamVersion.Major(), opts.upstreamVersion.Minor())
		},
		getWolfiPackageVersionComparison: func(opts commonOptions) string {
			return latestPatchComparison(opts.k0sVersion) // pause package version follows the k8s version
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
			Images: make(map[string]string),
		}

		k0sImages := config.ListK0sImages(k0sconfig.DefaultClusterConfig())

		if err := ApkoLogin(); err != nil {
			return fmt.Errorf("failed to apko login: %w", err)
		}

		wolfiAPKIndex, err := GetWolfiAPKIndex()
		if err != nil {
			return fmt.Errorf("failed to get APK index: %w", err)
		}

		for _, image := range k0sImages {
			logrus.Infof("updating image %s", image)

			upstreamVersion := TagFromImage(image)
			upstreamVersion = strings.TrimPrefix(upstreamVersion, "v")
			upstreamVersion = strings.Split(upstreamVersion, "-")[0]

			image = RemoveTagFromImage(image)

			componentName, ok := k0sImageComponents[image]
			if !ok {
				return fmt.Errorf("no component found for image %s", image)
			}

			component, ok := k0sComponents[componentName]
			if !ok {
				return fmt.Errorf("no component found for component name %s", componentName)
			}

			packageName, packageVersion, err := component.getPackageNameAndVersion(wolfiAPKIndex, upstreamVersion)
			if err != nil {
				return fmt.Errorf("failed to get package name and version for %s: %w", componentName, err)
			}

			logrus.Infof("building and publishing %s, %s=%s", componentName, packageName, packageVersion)

			if err := ApkoBuildAndPublish(componentName, packageName, packageVersion, upstreamVersion); err != nil {
				return fmt.Errorf("failed to apko build and publish for %s: %w", componentName, err)
			}

			digest, err := GetDigestFromBuildFile()
			if err != nil {
				return fmt.Errorf("failed to get digest from build file: %w", err)
			}

			newmeta.Images[componentName] = fmt.Sprintf("%s@%s", packageVersion, digest)
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
