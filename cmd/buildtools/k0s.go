package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	apkIndexURL = "https://packages.wolfi.dev/os/x86_64/APKINDEX.tar.gz"
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

		tmpdir, err := os.MkdirTemp(os.TempDir(), "k0s-images-*")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}

		if err := DownloadFile(apkIndexURL, filepath.Join(tmpdir, "APKINDEX.tar.gz")); err != nil {
			return fmt.Errorf("failed to download APKINDEX.tar.gz: %w", err)
		}

		if err := ExtractTarGz(filepath.Join(tmpdir, "APKINDEX.tar.gz"), tmpdir); err != nil {
			return fmt.Errorf("failed to extract APKINDEX.tar.gz: %w", err)
		}

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

		if err := runCommand("make", "bin/apko"); err != nil {
			return fmt.Errorf("failed to make bin/apko: %w", err)
		}

		for _, component := range k0sComponents {
			version, err := getPackageVersion(tmpdir, component.name)
			if err != nil {
				return fmt.Errorf("failed to get package version for %s: %w", component.name, err)
			}

			apkoConfig, err := generateApkoConfig(component.name, version, tmpdir)
			if err != nil {
				return fmt.Errorf("failed to generate apko config for %s: %w", component.name, err)
			}

			if err := runCommand(
				"make",
				"apko-build-and-publish",
				fmt.Sprintf("REGISTRY=%s", os.Getenv("REGISTRY_SERVER")),
				fmt.Sprintf("USERNAME=%s", os.Getenv("REGISTRY_USER")),
				fmt.Sprintf("PASSWORD=%s", os.Getenv("REGISTRY_PASS")),
				fmt.Sprintf("IMAGE=replicated/ec-%s:%s", component.name, version),
				fmt.Sprintf("APKO_CONFIG=%s", apkoConfig),
				fmt.Sprintf("VERSION=%s", version),
			); err != nil {
				return fmt.Errorf("failed to build and publish apko for %s: %w", component.name, err)
			}

			digest, err := getDigestFromBuildFile()
			if err != nil {
				return fmt.Errorf("failed to get digest from build file: %w", err)
			}

			if err := SetMakefileVariable(component.makefileVar, fmt.Sprintf("%s@%s", version, digest)); err != nil {
				return fmt.Errorf("failed to set %s version: %w", component.name, err)
			}
		}

		return nil
	},
}

func getPackageVersion(tmpdir, name string) (string, error) {
	output, err := exec.Command("pkg/goods/bins/k0s", "airgap", "list-images", "--all").Output()
	if err != nil {
		return "", fmt.Errorf("failed to list k0s images: %w", err)
	}

	// example output:
	// quay.io/k0sproject/calico-node:v3.26.1-1
	// quay.io/k0sproject/coredns:1.11.3
	// quay.io/k0sproject/apiserver-network-proxy-agent:v0.1.4

	pinnedVersion := ""
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
		pinnedVersion = strings.TrimPrefix(parts[1], "v")
		pinnedVersion = strings.Split(pinnedVersion, "-")[0]
		break
	}

	if pinnedVersion == "" {
		return "", fmt.Errorf("failed to find pinned version for %s", name)
	}

	apkIndex, err := os.ReadFile(filepath.Join(tmpdir, "APKINDEX"))
	if err != nil {
		return "", fmt.Errorf("failed to read APKINDEX: %w", err)
	}

	// example APKINDEX content:
	// P:calico-node
	// V:3.26.1-r1
	// ...
	//
	// P:calico-node
	// V:3.26.1-r10
	// ...
	//
	// P:calico-node
	// V:3.26.1-r9
	// ...

	var revisions []int
	scanner = bufio.NewScanner(bytes.NewReader(apkIndex))
	for scanner.Scan() {
		line := scanner.Text()
		// filter by package name
		if !strings.HasPrefix(line, "P:"+name) {
			continue
		}
		scanner.Scan()
		line = scanner.Text()
		// filter by package version
		if !strings.Contains(line, "V:"+pinnedVersion) {
			continue
		}
		// find the latest revision for the package version
		parts := strings.Split(line, "-r")
		if len(parts) != 2 {
			return "", fmt.Errorf("incorrect number of parts in APKINDEX line: %s", line)
		}
		revision, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", fmt.Errorf("failed to parse revision: %w", err)
		}
		revisions = append(revisions, revision)
	}

	if len(revisions) == 0 {
		return "", fmt.Errorf("failed to find package version for %s", name)
	}

	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i] > revisions[j]
	})

	return fmt.Sprintf("%s-r%d", pinnedVersion, revisions[0]), nil
}

func generateApkoConfig(name, version, destdir string) (string, error) {
	inputFile := filepath.Join("deploy", "images", name, "apko.tmpl.yaml")
	outputFile := filepath.Join(destdir, "apko.yaml")

	contents, err := os.ReadFile(inputFile)
	if err != nil {
		return "", fmt.Errorf("failed to read input file: %w", err)
	}

	updated := strings.ReplaceAll(string(contents), "__VERSION__", version)

	if err := os.WriteFile(outputFile, []byte(updated), 0644); err != nil {
		return "", fmt.Errorf("failed to write output file: %w", err)
	}
	return outputFile, nil
}

func getDigestFromBuildFile() (string, error) {
	contents, err := os.ReadFile("build/digest")
	if err != nil {
		return "", fmt.Errorf("failed to read build file: %w", err)
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
