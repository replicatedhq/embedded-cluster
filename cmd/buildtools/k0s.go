package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"

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
		getCustomImageName: func(opts addonComponentOptions) (string, error) {
			tag, err := getCalicoTag(opts)
			if err != nil {
				return "", fmt.Errorf("failed to get calico release: %w", err)
			}
			return fmt.Sprintf("registry.replicated.com/library/calico-node:%s", tag), nil
		},
	},
	"quay.io/k0sproject/calico-cni": {
		name: "calico-cni",
		getCustomImageName: func(opts addonComponentOptions) (string, error) {
			tag, err := getCalicoTag(opts)
			if err != nil {
				return "", fmt.Errorf("failed to get calico tag: %w", err)
			}
			return fmt.Sprintf("registry.replicated.com/library/calico-cni:%s", tag), nil
		},
	},
	"quay.io/k0sproject/calico-kube-controllers": {
		name: "calico-kube-controllers",
		getCustomImageName: func(opts addonComponentOptions) (string, error) {
			tag, err := getCalicoTag(opts)
			if err != nil {
				return "", fmt.Errorf("failed to get calico tag: %w", err)
			}
			return fmt.Sprintf("registry.replicated.com/library/calico-kube-controllers:%s", tag), nil
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
	"registry.k8s.io/pause":    pauseComponent,
	"quay.io/k0sproject/pause": pauseComponent,
	"quay.io/k0sproject/envoy-distroless": {
		name: "envoy-distroless",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return fmt.Sprintf("envoy-%d.%d", opts.upstreamVersion.Major(), opts.upstreamVersion.Minor())
		},
	},
}

var pauseComponent = addonComponent{
	name: "pause",
	getCustomImageName: func(opts addonComponentOptions) (string, error) {
		k0sConfig := k0sv1beta1.DefaultClusterConfig()
		pauseVersion := k0sConfig.Spec.Images.Pause.Version
		sv, err := semver.NewVersion(pauseVersion)
		if err != nil {
			return "", fmt.Errorf("failed to parse pause version: %w", err)
		}

		// Search the registry for the latest patch version for this major.minor
		latestTag, err := getLatestPauseImageTag(opts.ctx, sv.Major(), sv.Minor())
		if err != nil {
			return "", fmt.Errorf("failed to get latest pause image tag: %w", err)
		}

		return fmt.Sprintf("registry.k8s.io/pause:%s", latestTag), nil
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

		metaImages, err := UpdateImages(
			c.Context, k0sImageComponents, config.Metadata.Images, k0sImages, c.StringSlice("image"),
		)
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
	v, err := exec.Command("make", "print-K0S_VERSION").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get k0s version: %w", err)
	}
	return semver.MustParse(strings.TrimSpace(string(v))), nil
}

func getCalicoTag(opts addonComponentOptions) (string, error) {
	calicoVersion := getCalicoVersion(opts)
	constraints := mustParseSemverConstraints(latestPatchConstraint(calicoVersion))
	tag, err := GetGreatestGitHubTag(opts.ctx, "projectcalico", "calico", constraints)
	if err != nil {
		return "", fmt.Errorf("failed to get calico release: %w", err)
	}
	return tag, nil
}

func getCalicoVersion(opts addonComponentOptions) *semver.Version {
	// k0s versions prior to 1.31 use calico versions < 3.28,
	// but securebuild doesn't have versions prior to 3.28
	if opts.k0sVersion.LessThan(semver.MustParse("1.31")) {
		return semver.MustParse("3.28.0")
	}
	return opts.upstreamVersion
}

func getLatestPauseImageTag(ctx context.Context, major, minor uint64) (string, error) {
	// Query the registry for available pause image tags
	resp, err := http.Get("https://us-west2-docker.pkg.dev/v2/k8s-artifacts-prod/images/pause/tags/list")
	if err != nil {
		return "", fmt.Errorf("failed to query pause image registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry returned status: %d", resp.StatusCode)
	}

	var result struct {
		Tags []string `json:"tags"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode registry response: %w", err)
	}

	// Filter tags that match the major.minor version pattern (e.g., "3.9", "3.10")
	// and find the latest patch version
	var matchingTags []string
	prefix := fmt.Sprintf("%d.%d", major, minor)

	for _, tag := range result.Tags {
		// Skip non-version tags
		if tag == "go" || tag == "latest" || strings.HasPrefix(tag, "sha256-") || tag == "test" || tag == "test2" {
			continue
		}

		// Only include tags that start with our major.minor version
		if strings.HasPrefix(tag, prefix) {
			matchingTags = append(matchingTags, tag)
		}
	}

	if len(matchingTags) == 0 {
		return "", fmt.Errorf("no pause image tags found for version %d.%d", major, minor)
	}

	// Sort tags by version and return the latest
	sort.Slice(matchingTags, func(i, j int) bool {
		// Extract the patch version part
		verI := strings.TrimPrefix(matchingTags[i], prefix+".")
		verJ := strings.TrimPrefix(matchingTags[j], prefix+".")

		// If no patch version, treat as 0
		if verI == "" {
			verI = "0"
		}
		if verJ == "" {
			verJ = "0"
		}

		// Special case: if the tag exactly matches the prefix (e.g., "3.10" for prefix "3.10"),
		// it should be treated as having patch version 0
		if matchingTags[i] == prefix {
			verI = "0"
		}
		if matchingTags[j] == prefix {
			verJ = "0"
		}

		// Split by dots to handle versions like "10.1" vs "10"
		partsI := strings.Split(verI, ".")
		partsJ := strings.Split(verJ, ".")

		// Compare each part numerically
		maxLen := len(partsI)
		if len(partsJ) > maxLen {
			maxLen = len(partsJ)
		}

		for k := 0; k < maxLen; k++ {
			partI := 0
			partJ := 0

			if k < len(partsI) {
				fmt.Sscanf(partsI[k], "%d", &partI)
			}
			if k < len(partsJ) {
				fmt.Sscanf(partsJ[k], "%d", &partJ)
			}

			if partI != partJ {
				return partI < partJ
			}
		}

		// If all parts are equal, use string comparison as fallback
		return matchingTags[i] < matchingTags[j]
	})

	// Return the latest tag
	latestTag := matchingTags[len(matchingTags)-1]
	logrus.Infof("Selected pause image tag %s for version %d.%d", latestTag, major, minor)

	return latestTag, nil
}
