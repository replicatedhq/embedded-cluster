package main

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type addonComponent struct {
	getWolfiPackageName              func(opts addonComponentOptions) string
	getWolfiPackageVersionComparison func(opts addonComponentOptions) string
	upstreamVersionInputOverride     string
	useUpstreamImage                 bool
}

type addonComponentOptions struct {
	k0sVersion       *semver.Version
	upstreamVersion  *semver.Version
	latestK8sVersion *semver.Version
}

func (c *addonComponent) getPackageNameAndVersion(wolfiAPKIndex []byte, upstreamVersion string) (string, string, error) {
	packageName := ""
	if c.getWolfiPackageName == nil {
		return packageName, strings.TrimPrefix(upstreamVersion, "v"), nil
	}

	k0sVersion, err := getK0sVersion()
	if err != nil {
		return "", "", fmt.Errorf("get k0s version: %w", err)
	}

	latestK8sVersion, err := GetLatestKubernetesVersion()
	if err != nil {
		return "", "", fmt.Errorf("get latest k8s version: %w", err)
	}

	if c.getWolfiPackageName != nil {
		packageName = c.getWolfiPackageName(addonComponentOptions{
			k0sVersion:       k0sVersion,
			upstreamVersion:  semver.MustParse(upstreamVersion),
			latestK8sVersion: latestK8sVersion,
		})
	}

	comparison := latestPatchComparison(semver.MustParse(upstreamVersion))
	if c.getWolfiPackageVersionComparison != nil {
		comparison = c.getWolfiPackageVersionComparison(addonComponentOptions{
			k0sVersion:       k0sVersion,
			upstreamVersion:  semver.MustParse(upstreamVersion),
			latestK8sVersion: latestK8sVersion,
		})
	}
	constraints, err := semver.NewConstraint(comparison)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse version constraint: %w", err)
	}

	packageVersion, err := FindWolfiPackageVersion(wolfiAPKIndex, packageName, constraints)
	if err != nil {
		return "", "", fmt.Errorf("failed to find wolfi package version for %s=%s: %w", packageName, upstreamVersion, err)
	}

	return packageName, packageVersion, nil
}
