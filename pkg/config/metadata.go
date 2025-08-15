package config

import (
	"embed"
	"fmt"
	"io/fs"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"gopkg.in/yaml.v3"
)

var (
	_metadata   *release.K0sMetadata
	metadataMap = map[string]release.K0sMetadata{}

	//go:embed static/metadata-1_*.yaml
	metadataFS embed.FS

	metadataMinorRegex = regexp.MustCompile(`^metadata-1_(\d+)\.yaml$`)
)

func Metadata(ver string) release.K0sMetadata {
	sv, err := semver.NewVersion(ver)
	if err != nil {
		panic(fmt.Sprintf("failed to parse k0s version %s: %v", ver, err))
	}

	metadata, ok := metadataMap[fmt.Sprintf("%d", sv.Minor())]
	if !ok {
		panic(fmt.Sprintf("no metadata found for k0s version: %s", ver))
	}
	return metadata
}

func init() {
	if err := populateMetadataMap(); err != nil {
		panic(fmt.Errorf("failed to populate metadata map: %v", err))
	}

	if versions.K0sVersion != "0.0.0" {
		m := Metadata(versions.K0sVersion)
		_metadata = &m
	}
}

func populateMetadataMap() error {
	err := fs.WalkDir(metadataFS, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		minorVersion := metadataMinorRegex.FindStringSubmatch(d.Name())[1]
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
