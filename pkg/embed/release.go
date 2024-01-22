package embed

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	kyaml "sigs.k8s.io/yaml"
)

// ReleaseData holds the parsed data from a Kots Release.
type ReleaseData struct {
	data                  []byte
	Application           []byte
	HostPreflights        [][]byte
	License               []byte
	EmbeddedClusterConfig []byte
	ChannelRelease        []byte
}

// NewReleaseDataFrom parses the provide slice of bytes and returns a ReleaseData
// object. The slice of bytes is expected to be a tar.gz file.
func NewReleaseDataFrom(data []byte) (*ReleaseData, error) {
	rd := &ReleaseData{data: data}
	if len(data) == 0 {
		return rd, nil
	}
	if err := rd.parse(); err != nil {
		return nil, fmt.Errorf("unable to parse release data: %w", err)
	}
	return rd, nil
}

// GetHostPreflights returns a list of HostPreflight specs that are found in the binary.
// These are part of the embedded Kots Application Release.
func (r *ReleaseData) GetHostPreflights() (*v1beta2.HostPreflightSpec, error) {
	if len(r.HostPreflights) == 0 {
		return &v1beta2.HostPreflightSpec{}, nil
	}
	all := &v1beta2.HostPreflightSpec{}
	for _, serialized := range r.HostPreflights {
		spec, err := preflights.UnserializeSpec(serialized)
		if err != nil {
			return nil, fmt.Errorf("unable to unserialize preflight spec: %w", err)
		}
		all.Collectors = append(all.Collectors, spec.Collectors...)
		all.Analyzers = append(all.Analyzers, spec.Analyzers...)
	}
	return all, nil
}

// GetLicense reads the kots license from the embedded Kots Application Release. If no license
// is found, returns nil and no error.
func (r *ReleaseData) GetLicense() (*v1beta1.License, error) {
	if len(r.License) == 0 {
		return nil, nil
	}
	var license v1beta1.License
	if err := kyaml.Unmarshal(r.License, &license); err != nil {
		return nil, fmt.Errorf("unable to unmarshal license: %w", err)
	}
	return &license, nil
}

// GetApplication reads and returns the kots application embedded as part of the release. If
// no application is found, returns nil and no error. This function does not unmarshal the
// application yaml.
func (r *ReleaseData) GetApplication() ([]byte, error) {
	return r.Application, nil
}

// GetEmbeddedClusterConfig reads the embedded cluster config from the embedded Kots Application
// Release.
func (r *ReleaseData) GetEmbeddedClusterConfig() (*embeddedclusterv1beta1.Config, error) {
	if len(r.EmbeddedClusterConfig) == 0 {
		return nil, nil
	}
	var cfg embeddedclusterv1beta1.Config
	if err := kyaml.Unmarshal(r.EmbeddedClusterConfig, &cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal embedded cluster config: %w", err)
	}
	return &cfg, nil
}

// ChannelRelease contains information about a specific app release inside a channel.
type ChannelRelease struct {
	VersionLabel string `yaml:"versionLabel"`
}

// GetChannelRelease reads the embedded channel release object. If no channel release is found,
// returns nil and no error.
func (r *ReleaseData) GetChannelRelease() (*ChannelRelease, error) {
	if len(r.ChannelRelease) == 0 {
		return nil, nil
	}
	var release ChannelRelease
	if err := yaml.Unmarshal(r.ChannelRelease, &release); err != nil {
		return nil, fmt.Errorf("unable to unmarshal channel release: %w", err)
	}
	return &release, nil
}

// parse turns splits data property into the different parts of the release.
func (r *ReleaseData) parse() error {
	gzr, err := gzip.NewReader(bytes.NewReader(r.data))
	if err != nil {
		return fmt.Errorf("unable to create gzip reader: %w", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return fmt.Errorf("unable to read file: %w", err)
		case header == nil:
			continue
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		content := bytes.NewBuffer(nil)
		if _, err := io.Copy(content, tr); err != nil {
			return fmt.Errorf("unable to copy file out of tar: %w", err)
		}
		if bytes.Contains(content.Bytes(), []byte("apiVersion: kots.io/v1beta1")) {
			if bytes.Contains(content.Bytes(), []byte("kind: Application")) {
				r.Application = content.Bytes()
			}
			if bytes.Contains(content.Bytes(), []byte("kind: License")) {
				r.License = content.Bytes()
			}
			continue
		}
		if bytes.Contains(content.Bytes(), []byte("apiVersion: troubleshoot.sh/v1beta2")) {
			if !bytes.Contains(content.Bytes(), []byte("kind: HostPreflight")) {
				continue
			}
			if bytes.Contains(content.Bytes(), []byte("cluster.kurl.sh/v1beta1")) {
				continue
			}
			r.HostPreflights = append(r.HostPreflights, content.Bytes())
			continue
		}
		if bytes.Contains(content.Bytes(), []byte("apiVersion: embeddedcluster.replicated.com/v1beta1")) {
			if !bytes.Contains(content.Bytes(), []byte("kind: Config")) {
				continue
			}
			r.EmbeddedClusterConfig = content.Bytes()
			continue
		}
		if bytes.Contains(content.Bytes(), []byte("# channel release object")) {
			r.ChannelRelease = content.Bytes()
			continue
		}
	}
}
