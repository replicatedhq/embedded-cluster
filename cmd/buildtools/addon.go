package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
)

type addonComponent struct {
	name                         string
	getCustomImageName           func(opts addonComponentOptions) (string, error)
	upstreamVersionInputOverride string
	useUpstreamImage             bool
	// usePlainTag emits the image tag as-is (no "-<arch>@<digest>" suffix). Required for
	// the sandbox/pause image: containerd 2.x (k0s >= 1.36) pulls it by digest but then
	// resolves the sandbox image by its full reference, and a synthetic arch-suffixed tag
	// that does not exist in the registry fails that lookup ("failed to get sandbox image").
	usePlainTag bool
}

type addonComponentOptions struct {
	ctx              context.Context
	k0sVersion       *semver.Version
	upstreamVersion  *semver.Version
	latestK8sVersion *semver.Version
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
	return "", "", fmt.Errorf("no published image source configured for %s", c.name)
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

	repo := FamiliarImageName(RemoveTagFromImage(image))
	repo = addProxyAnonymousPrefix(repo)

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

	var tag string
	if c.usePlainTag {
		tag = TagFromImage(customImage)
	} else {
		digest, err := GetImageDigest(ctx, customImage, arch)
		if err != nil {
			return "", "", fmt.Errorf("failed to get image %s digest: %w", customImage, err)
		}
		tag = fmt.Sprintf("%s-%s@%s", TagFromImage(customImage), arch, digest)
	}

	repo := FamiliarImageName(RemoveTagFromImage(customImage))
	repo = addProxyAnonymousPrefix(repo)

	return repo, tag, nil
}
