package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"dagger/embedded-cluster/internal/dagger"
	"go.yaml.in/yaml/v3"
)

// Local and CI builds use the same recipes as SecureBuild, with the checked-out
// source and locally signed head packages instead of a remote release checkout.
func localPackageConfig(ctx context.Context, src *dagger.Directory, family, minor, version string) (*dagger.File, error) {
	minor, err := localKubernetesMinor(ctx, src, minor)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("securebuild/package/%s-k8s1.%s/melange.yaml", family, minor)
	contents, err := src.File(path).Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	contents, err = adaptLocalPackage(contents, version)
	if err != nil {
		return nil, fmt.Errorf("adapt %s: %w", path, err)
	}
	return dag.Directory().WithNewFile("melange.yaml", contents).File("melange.yaml"), nil
}

func localImageConfig(ctx context.Context, src *dagger.Directory, family, minor string) (*dagger.File, error) {
	minor, err := localKubernetesMinor(ctx, src, minor)
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-k8s1.%s", family, minor)
	path := fmt.Sprintf("securebuild/image/apko-%s.yaml", name)
	contents, err := src.File(path).Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	contents, err = adaptLocalImage(contents, name)
	if err != nil {
		return nil, fmt.Errorf("adapt %s: %w", path, err)
	}
	return dag.Directory().WithNewFile("apko.yaml", contents).File("apko.yaml"), nil
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

func adaptLocalPackage(contents, version string) (string, error) {
	var config map[string]any
	if err := yaml.Unmarshal([]byte(contents), &config); err != nil {
		return "", err
	}
	environment, ok := config["environment"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("package recipe has no environment")
	}
	variables, ok := environment["environment"].(map[string]any)
	if !ok {
		variables = map[string]any{}
		environment["environment"] = variables
	}
	variables["EC_VERSION"] = version
	variables["VERSION"] = version
	variables["GOCACHE"] = "/cache/melange/gocache"
	variables["GOMODCACHE"] = "/cache/melange/gomodcache"
	// The Makefiles use bash, including for older Kubernetes recipe variants.
	buildContents, ok := environment["contents"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("package recipe has no build contents")
	}
	buildPackages, _ := buildContents["packages"].([]any)
	buildContents["packages"] = append(buildPackages, "bash", "git")
	var pipeline []any
	steps, ok := config["pipeline"].([]any)
	if !ok {
		return "", fmt.Errorf("package recipe has no pipeline")
	}
	for _, entry := range steps {
		step, ok := entry.(map[string]any)
		if !ok {
			return "", fmt.Errorf("invalid package pipeline step")
		}
		if step["uses"] == "git-checkout" {
			continue
		}
		if runs, ok := step["runs"].(string); ok {
			// Keep the APK's high head version while embedding the actual CI or
			// prerelease version in the operator binary.
			step["runs"] = strings.ReplaceAll(runs, "${{package.version}}+${{vars.k8s-version}}", `"${EC_VERSION}"`)
		}
		pipeline = append(pipeline, step)
	}
	config["pipeline"] = pipeline
	data, err := yaml.Marshal(config)
	return string(data), err
}

func adaptLocalImage(contents, name string) (string, error) {
	var config map[string]any
	if err := yaml.Unmarshal([]byte(contents), &config); err != nil {
		return "", err
	}
	imageContents, ok := config["contents"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("image recipe has no contents")
	}
	repositories, _ := imageContents["repositories"].([]any)
	keyring, _ := imageContents["keyring"].([]any)
	imageContents["repositories"] = append([]any{"./packages"}, repositories...)
	imageContents["keyring"] = append([]any{"./melange.rsa.pub"}, keyring...)
	packages, ok := imageContents["packages"].([]any)
	if !ok {
		return "", fmt.Errorf("image recipe has no packages")
	}
	found := false
	for i, pkg := range packages {
		switch pkg {
		case name:
			packages[i] = name + "-head=1000.0.0-r0"
			found = true
		case name + "-compat":
			packages[i] = name + "-head-compat=1000.0.0-r0"
		}
	}
	if !found {
		return "", fmt.Errorf("image recipe does not reference %s", name)
	}
	data, err := yaml.Marshal(config)
	return string(data), err
}
