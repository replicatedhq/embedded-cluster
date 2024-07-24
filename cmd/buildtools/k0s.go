package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver/v3"
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
}

var k0sComponents = map[string]addonComponent{
	"coredns": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion *semver.Version) string {
			return "coredns"
		},
	},
	"calico-node": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion *semver.Version) string {
			return "calico-node"
		},
	},
	"calico-cni": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion *semver.Version) string {
			return "calico-cni"
		},
	},
	"calico-kube-controllers": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion *semver.Version) string {
			return "calico-kube-controllers"
		},
	},
	"metrics-server": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion *semver.Version) string {
			return "metrics-server"
		},
	},
	"kube-proxy": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion *semver.Version) string {
			return fmt.Sprintf("kube-proxy-%d.%d-default", k0sVersion.Major(), k0sVersion.Minor())
		},
		getWolfiPackageVersionComparison: func(k0sVersion *semver.Version, upstreamVersion *semver.Version) string {
			// match the greatest patch version of the same minor version
			return fmt.Sprintf(">=%d.%d, <%d.%d", k0sVersion.Major(), k0sVersion.Minor(), k0sVersion.Major(), k0sVersion.Minor()+1)
		},
	},
	"envoy-distroless": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion *semver.Version) string {
			return fmt.Sprintf("envoy-%d.%d", upstreamVersion.Major(), upstreamVersion.Minor())
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

		if err := makeK0s(); err != nil {
			return fmt.Errorf("failed to make k0s: %w", err)
		}

		images, err := listK0sImages()
		if err != nil {
			return fmt.Errorf("failed to list k0s images: %w", err)
		}

		k0sVersion, err := getK0sVersion()
		if err != nil {
			return fmt.Errorf("failed to get k0s version: %w", err)
		}

		if err := ApkoLogin(); err != nil {
			return fmt.Errorf("failed to apko login: %w", err)
		}

		wolfiAPKIndex, err := GetWolfiAPKIndex()
		if err != nil {
			return fmt.Errorf("failed to get APK index: %w", err)
		}

		for _, image := range images {
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

			packageName, packageVersion, err := component.getPackageNameAndVersion(wolfiAPKIndex, k0sVersion, upstreamVersion)
			if err != nil {
				return fmt.Errorf("failed to get package name and version for %s: %w", componentName, err)
			}

			logrus.Infof("building and publishing %s, %s=%s", componentName, packageName, packageVersion)

			if err := ApkoBuildAndPublish(componentName, packageName, packageVersion); err != nil {
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

func makeK0s() error {
	if v := os.Getenv("INPUT_K0S_VERSION"); v != "" {
		logrus.Infof("using input override from INPUT_K0S_VERSION: %s", v)
		cmd := exec.Command("make", "pkg/goods/bins/k0s", fmt.Sprintf("K0S_VERSION=%s", v), "K0S_BINARY_SOURCE_OVERRIDE=")
		if err := RunCommand(cmd); err != nil {
			return fmt.Errorf("make k0s: %w", err)
		}
	} else {
		cmd := exec.Command("make", "pkg/goods/bins/k0s")
		if err := RunCommand(cmd); err != nil {
			return fmt.Errorf("make k0s: %w", err)
		}
	}
	return nil
}

func listK0sImages() ([]string, error) {
	output, err := exec.Command("pkg/goods/bins/k0s", "airgap", "list-images", "--all").Output()
	if err != nil {
		return nil, fmt.Errorf("list k0s images: %w", err)
	}
	images := []string{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		image := scanner.Text()
		if _, ok := k0sImageComponents[RemoveTagFromImage(image)]; !ok {
			logrus.Warnf("skipping image %q as it is not in the list", image)
			continue
		}
		images = append(images, image)
	}
	return images, nil
}
