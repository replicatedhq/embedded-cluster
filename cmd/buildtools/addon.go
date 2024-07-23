package main

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

type addonComponent struct {
	getWolfiPackageName              func(k0sVersion *semver.Version, upstreamVersion string) string
	getWolfiPackageVersionComparison func(k0sVersion *semver.Version, upstreamVersion string) string
	upstreamVersionInputOverride     string
}

func (c *addonComponent) getPackageNameAndVersion(wolfiAPKIndex []byte, k0sVersion *semver.Version, upstreamVersion string) (string, string, error) {
	packageName := ""
	if c.getWolfiPackageName == nil {
		return packageName, upstreamVersion, nil
	}

	if c.getWolfiPackageName != nil {
		packageName = c.getWolfiPackageName(k0sVersion, upstreamVersion)
	}

	comparison := "=" + upstreamVersion
	if c.getWolfiPackageVersionComparison != nil {
		comparison = c.getWolfiPackageVersionComparison(k0sVersion, upstreamVersion)
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
