package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
)

type addonComponent struct {
	name                         string
	getCustomImageName           func(opts addonComponentOptions) (string, error)
	getWolfiPackageName          func(opts addonComponentOptions) string
	getWolfiPackageVersion       func(opts addonComponentOptions) string
	upstreamVersionInputOverride string
	useUpstreamImage             bool
}

type addonComponentOptions struct {
	ctx              context.Context
	k0sVersion       *semver.Version
	upstreamVersion  *semver.Version
	latestK8sVersion *semver.Version
}

func (c *addonComponent) resolveImageRepoAndTag(ctx context.Context, image string) (string, string, error) {
	if c.useUpstreamImage {
		return c.resolveUpstreamImageRepoAndTag(ctx, image)
	}
	if c.getCustomImageName != nil {
		return c.resolveCustomImageRepoAndTag(ctx, c.getUpstreamVersion(image))
	}
	return c.resolveApkoImageRepoAndTag(ctx, c.getUpstreamVersion(image))
}

func (c *addonComponent) getUpstreamVersion(image string) string {
	if c.upstreamVersionInputOverride != "" {
		if uv := os.Getenv(c.upstreamVersionInputOverride); uv != "" {
			logrus.Infof("using input override from %s: %s", c.upstreamVersionInputOverride, uv)
			return uv
		}
	}
	return TagFromImage(image)
}

func (c *addonComponent) resolveUpstreamImageRepoAndTag(ctx context.Context, image string) (string, string, error) {
	digest, err := GetImageDigest(ctx, image)
	if err != nil {
		return "", "", fmt.Errorf("failed to get image %s digest: %w", image, err)
	}
	tag := fmt.Sprintf("%s@%s", TagFromImage(image), digest)
	repo := FamiliarImageName(RemoveTagFromImage(image))
	return repo, tag, nil
}

func (c *addonComponent) resolveCustomImageRepoAndTag(ctx context.Context, upstreamVersion string) (string, string, error) {
	k0sVersion, err := getK0sVersion()
	if err != nil {
		return "", "", fmt.Errorf("get k0s version: %w", err)
	}
	latestK8sVersion, err := GetLatestKubernetesVersion()
	if err != nil {
		return "", "", fmt.Errorf("get latest k8s version: %w", err)
	}
	customImage, err := c.getCustomImageName(addonComponentOptions{
		ctx:              ctx,
		k0sVersion:       k0sVersion,
		upstreamVersion:  semver.MustParse(upstreamVersion),
		latestK8sVersion: latestK8sVersion,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to get image name for %s: %w", c.name, err)
	}
	digest, err := GetImageDigest(ctx, customImage)
	if err != nil {
		return "", "", fmt.Errorf("failed to get image %s digest: %w", customImage, err)
	}
	tag := fmt.Sprintf("%s@%s", TagFromImage(customImage), digest)
	repo := FamiliarImageName(RemoveTagFromImage(customImage))
	return repo, tag, nil
}

func (c *addonComponent) resolveApkoImageRepoAndTag(ctx context.Context, upstreamVersion string) (string, string, error) {
	packageName, packageVersion, err := c.getPackageNameAndVersion(ctx, upstreamVersion)
	if err != nil {
		return "", "", fmt.Errorf("failed to get package name and version constraint for %s: %w", c.name, err)
	}

	logrus.Infof("building and publishing %s", c.name)

	if err := ApkoBuildAndPublish(c.name, packageName, packageVersion); err != nil {
		return "", "", fmt.Errorf("failed to apko build and publish for %s: %w", c.name, err)
	}

	builtImage, err := GetImageNameFromBuildFile()
	if err != nil {
		return "", "", fmt.Errorf("failed to get digest from build file: %w", err)
	}

	parts := strings.SplitN(builtImage, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid image name: %s", builtImage)
	}

	return parts[0], parts[1], nil
}

func (c *addonComponent) getPackageNameAndVersion(ctx context.Context, upstreamVersion string) (string, string, error) {
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
			ctx:              ctx,
			k0sVersion:       k0sVersion,
			upstreamVersion:  semver.MustParse(upstreamVersion),
			latestK8sVersion: latestK8sVersion,
		})
	}

	packageVersion := latestPatchVersion(semver.MustParse(upstreamVersion))
	if c.getWolfiPackageVersion != nil {
		packageVersion = c.getWolfiPackageVersion(addonComponentOptions{
			ctx:              ctx,
			k0sVersion:       k0sVersion,
			upstreamVersion:  semver.MustParse(upstreamVersion),
			latestK8sVersion: latestK8sVersion,
		})
	}

	return packageName, packageVersion, nil
}

func latestPatchVersion(s *semver.Version) string {
	return fmt.Sprintf("%d.%d", s.Major(), s.Minor())
}

func chartURLTemplate(upstream string) string {
	return fmt.Sprintf("oci://{{ if not .IsAirgap }}{{ .ReplicatedProxyDomain }}/anonymous/{{ end }}%s", upstream)
}

func imageRegistryTemplate() string {
	return fmt.Sprintf("{{ if not .IsAirgap }}{{ .ReplicatedProxyDomain }}/anonymous/{{ end }}")
}
