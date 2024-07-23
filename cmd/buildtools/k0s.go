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

var k0sComponents = []struct {
	name        string
	makefileVar string
}{
	{
		name:        "coredns",
		makefileVar: "COREDNS_VERSION",
	},
	{
		name:        "calico-node",
		makefileVar: "CALICO_NODE_VERSION",
	},
	{
		name:        "calico-cni",
		makefileVar: "CALICO_CNI_VERSION",
	},
	{
		name:        "calico-kube-controllers",
		makefileVar: "CALICO_KUBE_CONTROLLERS_VERSION",
	},
	{
		name:        "metrics-server",
		makefileVar: "METRICS_SERVER_VERSION",
	},
}

var updateK0sImagesCommand = &cli.Command{
	Name:      "k0s",
	Usage:     "Updates the k0s images",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating k0s images")

		k0sVersion := os.Getenv("INPUT_K0S_VERSION")
		if k0sVersion != "" {
			logrus.Infof("using input override from INPUT_K0S_VERSION: %s", k0sVersion)
		}

		if err := makeK0s(k0sVersion); err != nil {
			return fmt.Errorf("failed to make k0s: %w", err)
		}

		if err := ApkoLogin(); err != nil {
			return fmt.Errorf("failed to apko login: %w", err)
		}

		wolfiAPKIndex, err := GetWolfiAPKIndex()
		if err != nil {
			return fmt.Errorf("failed to get APK index: %w", err)
		}

		for _, component := range k0sComponents {
			upstreamVersion, err := getUpstreamVersion(component.name)
			if err != nil {
				return fmt.Errorf("failed to get upstream version for %s: %w", component.name, err)
			}

			constraints, err := semver.NewConstraint("=" + upstreamVersion)
			if err != nil {
				return fmt.Errorf("failed to parse version constraint: %w", err)
			}

			packageVersion, err := FindWolfiPackageVersion(wolfiAPKIndex, component.name, constraints)
			if err != nil {
				return fmt.Errorf("failed to get package version for %s: %w", component.name, err)
			}

			logrus.Infof("building and publishing %s=%s", component.name, packageVersion)

			if err := ApkoBuildAndPublish(component.name, "", packageVersion); err != nil {
				return fmt.Errorf("failed to apko build and publish for %s: %w", component.name, err)
			}

			digest, err := GetDigestFromBuildFile()
			if err != nil {
				return fmt.Errorf("failed to get digest from build file: %w", err)
			}

			if err := SetMakefileVariable(component.makefileVar, fmt.Sprintf("%s@%s", packageVersion, digest)); err != nil {
				return fmt.Errorf("failed to set %s version: %w", component.name, err)
			}
		}

		return nil
	},
}

func makeK0s(k0sVersion string) error {
	if k0sVersion != "" {
		cmd := exec.Command("make", "pkg/goods/bins/k0s", fmt.Sprintf("K0S_VERSION=%s", k0sVersion), "K0S_BINARY_SOURCE_OVERRIDE=")
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

func getUpstreamVersion(name string) (string, error) {
	output, err := exec.Command("pkg/goods/bins/k0s", "airgap", "list-images", "--all").Output()
	if err != nil {
		return "", fmt.Errorf("list k0s images: %w", err)
	}

	// example output:
	// quay.io/k0sproject/calico-node:v3.26.1-1
	// quay.io/k0sproject/coredns:1.11.3
	// quay.io/k0sproject/apiserver-network-proxy-agent:v0.1.4

	version := ""
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "/"+name+":") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			return "", fmt.Errorf("incorrect number of parts in image line: %s", line)
		}
		version = strings.TrimPrefix(parts[1], "v")
		version = strings.Split(version, "-")[0]
		break
	}

	if version == "" {
		return "", fmt.Errorf("%q image not found", name)
	}
	return version, nil
}
