package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver/v3"
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
}

var k0sComponents = map[string]addonComponent{
	"coredns": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion string) string {
			return "coredns"
		},
		makefileVar: "COREDNS_VERSION",
	},
	"calico-node": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion string) string {
			return "calico-node"
		},
		makefileVar: "CALICO_NODE_VERSION",
	},
	"calico-cni": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion string) string {
			return "calico-cni"
		},
		makefileVar: "CALICO_CNI_VERSION",
	},
	"calico-kube-controllers": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion string) string {
			return "calico-kube-controllers"
		},
		makefileVar: "CALICO_KUBE_CONTROLLERS_VERSION",
	},
	"metrics-server": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion string) string {
			return "metrics-server"
		},
		makefileVar: "METRICS_SERVER_VERSION",
	},
	"kube-proxy": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion string) string {
			return fmt.Sprintf("kube-proxy-%d.%d-default", k0sVersion.Major(), k0sVersion.Minor())
		},
		getWolfiPackageVersionComparison: func(k0sVersion *semver.Version, upstreamVersion string) string {
			// match the greatest patch version of the same minor version
			return fmt.Sprintf(">=%d.%d, <%d.%d", k0sVersion.Major(), k0sVersion.Minor(), k0sVersion.Major(), k0sVersion.Minor()+1)
		},
		makefileVar: "KUBE_PROXY_VERSION",
	},
}

var updateK0sImagesCommand = &cli.Command{
	Name:      "k0s",
	Usage:     "Updates the k0s images",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating k0s images")

		rawK0sVersion := os.Getenv("INPUT_K0S_VERSION")
		if rawK0sVersion != "" {
			logrus.Infof("using input override from INPUT_K0S_VERSION: %s", rawK0sVersion)
		} else {
			rawver, err := GetMakefileVariable("K0S_VERSION")
			if err != nil {
				return fmt.Errorf("failed to get k0s version: %w", err)
			}
			rawK0sVersion = rawver
		}

		images, err := listK0sImages(rawK0sVersion)
		if err != nil {
			return fmt.Errorf("failed to make k0s: %w", err)
		}

		k0sVersion := semver.MustParse(rawK0sVersion)

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

			if err := SetMakefileVariable(component.makefileVar, fmt.Sprintf("%s@%s", packageVersion, digest)); err != nil {
				return fmt.Errorf("failed to set %s version: %w", componentName, err)
			}
		}

		return nil
	},
}

func listK0sImages(k0sVersion string) ([]string, error) {
	cmd := exec.Command("make", "pkg/goods/bins/k0s", fmt.Sprintf("K0S_VERSION=%s", k0sVersion))
	if err := RunCommand(cmd); err != nil {
		return nil, fmt.Errorf("make k0s: %w", err)
	}

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
