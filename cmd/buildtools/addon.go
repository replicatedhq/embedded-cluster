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

func (c *addonComponent) buildImage(ctx context.Context, image string, archs string) (string, error) {
	if c.useUpstreamImage || c.getCustomImageName != nil {
		return "", nil
	}
	builtImage, err := c.buildApkoImage(ctx, image, archs)
	if err != nil {
		return "", fmt.Errorf("build apko image: %w", err)
	}
	return builtImage, nil
}

func (c *addonComponent) resolveImageRepoAndTag(ctx context.Context, image string, arch string) (repo string, tag string, err error) {
	if c.useUpstreamImage {
		repo, tag, err = c.resolveUpstreamImageRepoAndTag(ctx, image, arch)
		if err != nil {
			err = fmt.Errorf("resolve upstream image repo and tag: %w", err)
		}
		return
	}
	if c.getCustomImageName != nil {
		repo, tag, err = c.resolveCustomImageRepoAndTag(ctx, image, arch)
		if err != nil {
			err = fmt.Errorf("resolve custom image repo and tag: %w", err)
		}
		return
	}
	repo, tag, err = c.resolveApkoImageRepoAndTag(ctx, image, arch)
	if err != nil {
		err = fmt.Errorf("resolve apko image repo and tag: %w", err)
	}
	return
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

func (c *addonComponent) resolveUpstreamImageRepoAndTag(ctx context.Context, image string, arch string) (string, string, error) {
	digest, err := GetImageDigest(ctx, image, arch)
	if err != nil {
		return "", "", fmt.Errorf("failed to get image %s digest: %w", image, err)
	}
	tag := fmt.Sprintf("%s-%s@%s", TagFromImage(image), arch, digest)
	repo := fmt.Sprintf("proxy.replicated.com/anonymous/%s", FamiliarImageName(RemoveTagFromImage(image)))
	return repo, tag, nil
}

func (c *addonComponent) resolveCustomImageRepoAndTag(ctx context.Context, image string, arch string) (string, string, error) {
	upstreamVersion := c.getUpstreamVersion(image)

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
	digest, err := GetImageDigest(ctx, customImage, arch)
	if err != nil {
		return "", "", fmt.Errorf("failed to get image %s digest: %w", customImage, err)
	}
	tag := fmt.Sprintf("%s-%s@%s", TagFromImage(customImage), arch, digest)
	repo := fmt.Sprintf("proxy.replicated.com/anonymous/%s", FamiliarImageName(RemoveTagFromImage(customImage)))
	return repo, tag, nil
}

func (c *addonComponent) resolveApkoImageRepoAndTag(ctx context.Context, image string, arch string) (string, string, error) {
	builtImage, err := GetImageNameFromBuildFile("build/image")
	if err != nil {
		return "", "", fmt.Errorf("failed to get digest from build file: %w", err)
	}

	digest, err := GetImageDigest(ctx, builtImage, arch)
	if err != nil {
		return "", "", fmt.Errorf("failed to get image %s digest: %w", builtImage, err)
	}
	tag := fmt.Sprintf("%s-%s@%s", TagFromImage(builtImage), arch, digest)
	repo := fmt.Sprintf("proxy.replicated.com/anonymous/%s", FamiliarImageName(RemoveTagFromImage(builtImage)))
	return repo, tag, nil
}

func (c *addonComponent) buildApkoImage(ctx context.Context, image string, archs string) (string, error) {
	upstreamVersion := c.getUpstreamVersion(image)

	packageName, packageVersion, err := c.getPackageNameAndVersion(ctx, upstreamVersion)
	if err != nil {
		return "", fmt.Errorf("get package name and version constraint for %s: %w", c.name, err)
	}

	logrus.Infof("building and publishing %s", c.name)

	if err := ApkoLogin(); err != nil {
		return "", fmt.Errorf("apko login: %w", err)
	}

	if err := ApkoBuildAndPublish(c.name, packageName, packageVersion, archs); err != nil {
		return "", fmt.Errorf("apko build and publish for %s: %w", c.name, err)
	}

	builtImage, err := GetImageNameFromBuildFile("build/image")
	if err != nil {
		return "", fmt.Errorf("get image name from build file: %w", err)
	}
	return builtImage, nil
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
