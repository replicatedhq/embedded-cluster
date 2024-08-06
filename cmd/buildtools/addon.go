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

func (c *addonComponent) resolveImageAndTag(ctx context.Context, image string) (string, string, error) {
	if c.useUpstreamImage {
		return c.resolveUpstreamImageAndTag(ctx, image)
	}
	if c.getCustomImageName != nil {
		return c.resolveCustomImageAndTag(ctx, c.getUpstreamVersion(ctx, image))
	}
	return c.resolveApkoImageAndTag(ctx, c.getUpstreamVersion(ctx, image))
}

func (c *addonComponent) getUpstreamVersion(ctx context.Context, image string) string {
	if c.upstreamVersionInputOverride != "" {
		if uv := os.Getenv(c.upstreamVersionInputOverride); uv != "" {
			logrus.Infof("using input override from %s: %s", c.upstreamVersionInputOverride, uv)
			return uv
		}
	}
	return TagFromImage(image)
}

func (c *addonComponent) resolveUpstreamImageAndTag(ctx context.Context, image string) (string, string, error) {
	digest, err := GetImageDigest(ctx, image)
	if err != nil {
		return "", "", fmt.Errorf("failed to get image %s digest: %w", image, err)
	}
	tag := fmt.Sprintf("%s@%s", TagFromImage(image), digest)
	proxiedImage := fmt.Sprintf("proxy.replicated.com/anonymous/%s", RemoveTagFromImage(image))
	return proxiedImage, tag, nil
}

func (c *addonComponent) resolveCustomImageAndTag(ctx context.Context, upstreamVersion string) (string, string, error) {
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
	proxiedImage := fmt.Sprintf("proxy.replicated.com/anonymous/%s", RemoveTagFromImage(customImage))
	return proxiedImage, tag, nil
}

func (c *addonComponent) resolveApkoImageAndTag(ctx context.Context, upstreamVersion string) (string, string, error) {
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

	return fmt.Sprintf("proxy.replicated.com/anonymous/%s", parts[0]), parts[1], nil
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

	packageVersion := latestPatchConstraint(semver.MustParse(upstreamVersion))
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
