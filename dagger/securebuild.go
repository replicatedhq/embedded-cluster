package main

import (
	"context"
	"fmt"
	"regexp"

	"dagger/embedded-cluster/internal/dagger"
)

// Select the checked-in SecureBuild recipes without modifying their contents.
func localPackageConfig(ctx context.Context, src *dagger.Directory, family, minor string) (*dagger.File, error) {
	minor, err := localKubernetesMinor(ctx, src, minor)
	if err != nil {
		return nil, err
	}
	return src.File(fmt.Sprintf("securebuild/package/%s-k8s1.%s/melange.yaml", family, minor)), nil
}

func localImageConfig(ctx context.Context, src *dagger.Directory, family, minor string) (*dagger.File, error) {
	minor, err := localKubernetesMinor(ctx, src, minor)
	if err != nil {
		return nil, err
	}
	return src.File(fmt.Sprintf("securebuild/image/apko-%s-k8s1.%s.yaml", family, minor)), nil
}

func localKubernetesMinor(ctx context.Context, src *dagger.Directory, minor string) (string, error) {
	if minor != "" {
		return minor, nil
	}
	versions, err := src.File("versions.mk").Contents(ctx)
	if err != nil {
		return "", err
	}
	match := regexp.MustCompile(`(?m)^K0S_MINOR_VERSION\s*\?=\s*(\d+)`).FindStringSubmatch(versions)
	if len(match) != 2 {
		return "", fmt.Errorf("K0S_MINOR_VERSION not found in versions.mk")
	}
	return match[1], nil
}
