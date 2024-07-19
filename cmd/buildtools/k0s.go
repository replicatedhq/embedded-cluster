package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "k0s-version",
			Usage: "The version of k0s to use to determine image versions",
		},
	},
	Action: func(c *cli.Context) error {
		logrus.Infof("updating k0s images")

		k0sVersion := c.String("k0s-version")
		if k0sVersion != "" {
			if err := runCommand("make", "pkg/goods/bins/k0s", fmt.Sprintf("K0S_VERSION=%s", k0sVersion), "K0S_BINARY_SOURCE_OVERRIDE="); err != nil {
				return fmt.Errorf("failed to make k0s: %w", err)
			}
		} else {
			if err := runCommand("make", "pkg/goods/bins/k0s"); err != nil {
				return fmt.Errorf("failed to make k0s: %w", err)
			}
		}

		if err := runCommand("make", "apko"); err != nil {
			return fmt.Errorf("failed to make apko: %w", err)
		}

		if os.Getenv("REGISTRY_PASS") != "" {
			if err := runCommand(
				"make",
				"apko-login",
				fmt.Sprintf("REGISTRY=%s", os.Getenv("REGISTRY_SERVER")),
				fmt.Sprintf("USERNAME=%s", os.Getenv("REGISTRY_USER")),
				fmt.Sprintf("PASSWORD=%s", os.Getenv("REGISTRY_PASS")),
			); err != nil {
				return fmt.Errorf("failed to apko login: %w", err)
			}
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

			packageVersion, err := GetWolfiPackageVersion(wolfiAPKIndex, component.name, upstreamVersion)
			if err != nil {
				return fmt.Errorf("failed to get package version for %s: %w", component.name, err)
			}

			if err := runCommand(
				"make",
				"apko-build-and-publish",
				fmt.Sprintf("IMAGE=%s/replicated/ec-%s:%s", os.Getenv("REGISTRY_SERVER"), component.name, packageVersion),
				fmt.Sprintf("APKO_CONFIG=%s", filepath.Join("deploy", "images", component.name, "apko.tmpl.yaml")),
				fmt.Sprintf("PACKAGE_VERSION=%s", packageVersion),
			); err != nil {
				return fmt.Errorf("failed to build and publish apko for %s: %w", component.name, err)
			}

			digest, err := getDigestFromBuildFile()
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

func getDigestFromBuildFile() (string, error) {
	contents, err := os.ReadFile("build/digest")
	if err != nil {
		return "", fmt.Errorf("read build file: %w", err)
	}
	parts := strings.Split(string(contents), "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("incorrect number of parts in build file")
	}
	return strings.TrimSpace(parts[1]), nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
