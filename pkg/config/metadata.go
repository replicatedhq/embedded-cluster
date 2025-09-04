package config

import (
	"embed"
	"fmt"
	"io/fs"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"go.yaml.in/yaml/v3"
)

var (
	_metadata   *release.K0sMetadata
	metadataMap = map[string]release.K0sMetadata{}

	//go:embed static/metadata-1_*.yaml
	metadataFS embed.FS

	metadataMinorRegex = regexp.MustCompile(`^metadata-1_(\d+)\.yaml$`)
)

// Metadata returns the metadata for the given k0s minor version.
func Metadata(minorVersion string) release.K0sMetadata {
	metadata, ok := metadataMap[minorVersion]
	if !ok {
		panic(fmt.Sprintf("no metadata found for k0s version: %s", minorVersion))
	}
	return metadata
}

func init() {
	if err := populateMetadataMap(); err != nil {
		panic(fmt.Errorf("failed to populate metadata map: %v", err))
	}

	k8sVersion, err := semver.NewVersion(constant.KubernetesMajorMinorVersion)
	if err != nil {
		panic(fmt.Errorf("failed to parse kubernetes version %s: %v", constant.KubernetesMajorMinorVersion, err))
	}

	if versions.K0sVersion != "0.0.0" {
		// validate that the go mod dependency matches the K0S_VERSION specified at compile time
		if err := validateK0sVersion(k8sVersion); err != nil {
			panic(err)
		}
	}

	m := Metadata(fmt.Sprintf("%d", k8sVersion.Minor()))
	_metadata = &m
}

func validateK0sVersion(k8sVersion *semver.Version) error {
	sv, err := semver.NewVersion(versions.K0sVersion)
	if err != nil {
		return fmt.Errorf("failed to parse k0s version %s: %v", versions.K0sVersion, err)
	}

	if sv.Major() != k8sVersion.Major() || sv.Minor() != k8sVersion.Minor() {
		return fmt.Errorf("versions.K0sVersion %s does not match k0s go mod dependency version %s", sv.Original(), k8sVersion.Original())
	}
	return nil
}

func populateMetadataMap() error {
	err := fs.WalkDir(metadataFS, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		matches := metadataMinorRegex.FindStringSubmatch(d.Name())
		if len(matches) < 2 {
			return fmt.Errorf("filename %s does not match expected metadata pattern", d.Name())
		}
		minorVersion := matches[1]
		var metadata release.K0sMetadata
		content, err := metadataFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read file %s: %v", path, err)
		}
		if err := yaml.Unmarshal(content, &metadata); err != nil {
			return fmt.Errorf("unmarshal metadata file %s: %v", path, err)
		}
		metadataMap[minorVersion] = metadata
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk metadata files: %v", err)
	}
	return nil
}
