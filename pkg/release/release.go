package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"sync"

	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-utils/pkg/embed"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
	kyaml "sigs.k8s.io/yaml"
)

var (
	mtx         sync.Mutex
	releaseData *ReleaseData
)

// ReleaseData holds the parsed data from a Kots Release.
type ReleaseData struct {
	data                  []byte
	Application           []byte
	HostPreflights        [][]byte
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

// parseReleaseDataFromBinary reads the embedded data from the binary and sets the global
// releaseData variable only once.
func parseReleaseDataFromBinary() error {
	mtx.Lock()
	defer mtx.Unlock()
	if releaseData != nil {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to get executable path: %w", err)
	}
	data, err := embed.ExtractReleaseDataFromBinary(exe)
	if err != nil {
		return fmt.Errorf("failed to extract data from binary: %w", err)
	}
	release, err := NewReleaseDataFrom(data)
	if err != nil {
		return fmt.Errorf("failed to parse release data: %w", err)
	}
	releaseData = release
	return nil
}

// GetHostPreflights returns a list of HostPreflight specs that are found in the
// binary. These are part of the embedded Kots Application Release.
func GetHostPreflights() (*v1beta2.HostPreflightSpec, error) {
	if err := parseReleaseDataFromBinary(); err != nil {
		return nil, fmt.Errorf("failed to parse data from binary: %w", err)
	}
	return releaseData.GetHostPreflights()
}

// GetHostPreflights returns a list of HostPreflight specs that are found in the binary.
// These are part of the embedded Kots Application Release.
func (r *ReleaseData) GetHostPreflights() (*v1beta2.HostPreflightSpec, error) {
	if len(r.HostPreflights) == 0 {
		return &v1beta2.HostPreflightSpec{}, nil
	}
	all := &v1beta2.HostPreflightSpec{}
	for _, serialized := range r.HostPreflights {
		spec, err := unserializeHostPreflightSpec(serialized)
		if err != nil {
			return nil, fmt.Errorf("unable to unserialize preflight spec: %w", err)
		}
		all.Collectors = append(all.Collectors, spec.Collectors...)
		all.Analyzers = append(all.Analyzers, spec.Analyzers...)
	}
	return all, nil
}

// unserializeHostPreflightSpec unserializes a HostPreflightSpec from a raw slice of bytes.
func unserializeHostPreflightSpec(data []byte) (*v1beta2.HostPreflightSpec, error) {
	scheme := kruntime.NewScheme()
	if err := v1beta2.AddToScheme(scheme); err != nil {
		return nil, err
	}
	decoder := conversion.NewDecoder(scheme)
	var hpf v1beta2.HostPreflight
	if err := decoder.DecodeInto(data, &hpf); err != nil {
		return nil, err
	}
	return &hpf.Spec, nil
}

// GetApplication reads and returns the kots application embedded as part of the
// release. If no application is found, returns nil and no error. This function does
// not unmarshal the application yaml.
func GetApplication() ([]byte, error) {
	if err := parseReleaseDataFromBinary(); err != nil {
		return nil, fmt.Errorf("failed to parse data from binary: %w", err)
	}
	return releaseData.GetApplication()
}

// GetApplication reads and returns the kots application embedded as part of the release. If
// no application is found, returns nil and no error. This function does not unmarshal the
// application yaml.
func (r *ReleaseData) GetApplication() ([]byte, error) {
	return r.Application, nil
}

// GetEmbeddedClusterConfig reads the embedded cluster config from the embedded Kots
// Application Release.
func GetEmbeddedClusterConfig() (*embeddedclusterv1beta1.Config, error) {
	if err := parseReleaseDataFromBinary(); err != nil {
		return nil, fmt.Errorf("failed to parse data from binary: %w", err)
	}
	return releaseData.GetEmbeddedClusterConfig()
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
	ChannelID    string `yaml:"channelID"`
	ChannelSlug  string `yaml:"channelSlug"`
	AppSlug      string `yaml:"appSlug"`
}

// GetChannelRelease reads the embedded channel release object. If no channel release
// is found, returns nil and no error.
func GetChannelRelease() (*ChannelRelease, error) {
	if err := parseReleaseDataFromBinary(); err != nil {
		return nil, fmt.Errorf("failed to parse data from binary: %w", err)
	}
	return releaseData.GetChannelRelease()
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
		fmt.Println("+++ content", content.String())
		if _, err := io.Copy(content, tr); err != nil {
			return fmt.Errorf("unable to copy file out of tar: %w", err)
		}
		if bytes.Contains(content.Bytes(), []byte("apiVersion: kots.io/v1beta1")) {
			if bytes.Contains(content.Bytes(), []byte("kind: Application")) {
				r.Application = content.Bytes()
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

// SetReleaseDataForTests should only be called from tests. It sets the release information based on the supplied data.
func SetReleaseDataForTests(data map[string][]byte) error {
	mtx.Lock()
	defer mtx.Unlock()

	buf := bytes.NewBuffer([]byte{})
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	for name, content := range data {
		err := tw.WriteHeader(&tar.Header{
			Name: "name",
			Size: int64(len(content)),
		})
		if err != nil {
			return fmt.Errorf("unable to write header for %s: %w", name, err)
		}
		_, err = tw.Write(content)
		if err != nil {
			return fmt.Errorf("unable to write content for %s: %w", name, err)
		}
	}
	err := tw.Close()
	if err != nil {
		return fmt.Errorf("unable to close tar writer: %w", err)
	}
	err = gw.Close()
	if err != nil {
		return fmt.Errorf("unable to close gzip writer: %w", err)
	}

	rd, err := NewReleaseDataFrom(buf.Bytes())
	if err != nil {
		return fmt.Errorf("unable to set release data for tests: %w", err)
	}
	releaseData = rd
	return nil
}
