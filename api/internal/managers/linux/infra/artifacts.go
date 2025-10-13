// file reference from KOTS: https://github.com/replicatedhq/kots/blob/d26ebd2acaccc54313e7f7d5ca3ca580ae1a0bc5/pkg/imageutil/image.go#L329-L387
package infra

import (
	"fmt"
	"path/filepath"
	"strings"

	dockerref "github.com/containers/image/v5/docker/reference"
)

// A valid tag must be valid ASCII and can contain lowercase and uppercase letters, digits, underscores, periods, and hyphens.
// It can't start with a period or hyphen and must be no longer than 128 characters.
// ref: https://docs.docker.com/reference/cli/docker/image/tag/#description
func sanitizeTag(tag string) string {
	tag = strings.Join(dockerref.TagRegexp.FindAllString(tag, -1), "")
	if len(tag) > 128 {
		tag = tag[:128]
	}
	return tag
}

// A valid repo may contain lowercase letters, digits and separators.
// A separator is defined as a period, one or two underscores, or one or more hyphens.
// A component may not start or end with a separator.
// ref: https://docs.docker.com/reference/cli/docker/image/tag/#description
func sanitizeRepo(repo string) string {
	repo = strings.ToLower(repo)
	repo = strings.Join(dockerref.NameRegexp.FindAllString(repo, -1), "")
	return repo
}

type OCIArtifactPath struct {
	Name              string
	RegistryHost      string
	RegistryNamespace string
	Repository        string
	Tag               string
}

func (p *OCIArtifactPath) String() string {
	if p.RegistryNamespace == "" {
		return fmt.Sprintf("%s:%s", filepath.Join(p.RegistryHost, p.Repository), p.Tag)
	}
	return fmt.Sprintf("%s:%s", filepath.Join(p.RegistryHost, p.RegistryNamespace, p.Repository), p.Tag)
}

type ECArtifactOCIPathOptions struct {
	RegistryHost      string
	RegistryNamespace string
	ChannelID         string
	UpdateCursor      string
	VersionLabel      string
}

// newECOCIArtifactPath returns the OCI path for an embedded cluster artifact given
// the artifact filename and details about the configured registry and channel release.
func newECOCIArtifactPath(filename string, opts ECArtifactOCIPathOptions) *OCIArtifactPath {
	name := filepath.Base(filename)
	repository := filepath.Join("embedded-cluster", sanitizeRepo(name))
	tag := sanitizeTag(fmt.Sprintf("%s-%s-%s", opts.ChannelID, opts.UpdateCursor, opts.VersionLabel))
	return &OCIArtifactPath{
		Name:              name,
		RegistryHost:      opts.RegistryHost,
		RegistryNamespace: opts.RegistryNamespace,
		Repository:        repository,
		Tag:               tag,
	}
}
