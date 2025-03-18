package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/utils/pkg/embed"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"gopkg.in/yaml.v2"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
	kyaml "sigs.k8s.io/yaml"
)

var (
	_releaseData *ReleaseData
)

// ReleaseData holds the parsed data from a Kots Release.
type ReleaseData struct {
	data                  []byte
	Application           []byte
	HostPreflights        *v1beta2.HostPreflightSpec
	EmbeddedClusterConfig *embeddedclusterv1beta1.Config
	ChannelRelease        *ChannelRelease
	VeleroBackup          *velerov1.Backup
	VeleroRestore         *velerov1.Restore
}

// GetHostPreflights returns a list of HostPreflight specs that are found in the
// binary. These are part of the embedded Kots Application Release.
func GetHostPreflights() *v1beta2.HostPreflightSpec {
	return _releaseData.HostPreflights
}

// GetApplication reads and returns the kots application embedded as part of the
// release. If no application is found, returns nil and no error. This function does
// not unmarshal the application yaml.
func GetApplication() []byte {
	return _releaseData.Application
}

// GetEmbeddedClusterConfig reads the embedded cluster config from the embedded Kots
// Application Release.
func GetEmbeddedClusterConfig() *embeddedclusterv1beta1.Config {
	return _releaseData.EmbeddedClusterConfig
}

// GetVeleroBackup reads and returns the velero backup embedded as part of the release. If
// no backup is found, returns nil and no error.
func GetVeleroBackup() *velerov1.Backup {
	return _releaseData.VeleroBackup
}

// GetVeleroRestore reads and returns the velero restore embedded as part of the release. If
// no restore is found, returns nil and no error.
func GetVeleroRestore() *velerov1.Restore {
	return _releaseData.VeleroRestore
}

// GetChannelRelease reads the embedded channel release object. If no channel release
// is found, returns nil and no error.
func GetChannelRelease() *ChannelRelease {
	return _releaseData.ChannelRelease
}

func init() {
	rd, err := parseReleaseDataFromBinary()
	if err != nil {
		panic(fmt.Sprintf("Failed to parse release data from binary: %v", err))
	}
	_releaseData = rd
}

// newReleaseDataFrom parses the provide slice of bytes and returns a ReleaseData
// object. The slice of bytes is expected to be a tar.gz file.
func newReleaseDataFrom(data []byte) (*ReleaseData, error) {
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
func parseReleaseDataFromBinary() (*ReleaseData, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("unable to get executable path: %w", err)
	}
	data, err := embed.ExtractReleaseDataFromBinary(exe)
	if err != nil {
		return nil, fmt.Errorf("failed to extract data from binary: %w", err)
	}
	release, err := newReleaseDataFrom(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse release data: %w", err)
	}
	return release, nil
}

func parseHostPreflights(data []byte) (*v1beta2.HostPreflightSpec, error) {
	if len(data) == 0 {
		return nil, nil
	}
	return unserializeHostPreflightSpec(data)
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

func parseEmbeddedClusterConfig(data []byte) (*embeddedclusterv1beta1.Config, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var cfg embeddedclusterv1beta1.Config
	if err := kyaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal embedded cluster config: %w", err)
	}
	return &cfg, nil
}

func parseVeleroBackup(data []byte) (*velerov1.Backup, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var backup velerov1.Backup
	if err := kyaml.Unmarshal(data, &backup); err != nil {
		return nil, fmt.Errorf("unable to unmarshal velero backup: %w", err)
	}
	return &backup, nil
}

func parseVeleroRestore(data []byte) (*velerov1.Restore, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var restore velerov1.Restore
	if err := kyaml.Unmarshal(data, &restore); err != nil {
		return nil, fmt.Errorf("unable to unmarshal velero restore: %w", err)
	}
	return &restore, nil
}

// ChannelRelease contains information about a specific app release inside a channel.
type ChannelRelease struct {
	VersionLabel string `yaml:"versionLabel"`
	ChannelID    string `yaml:"channelID"`
	ChannelSlug  string `yaml:"channelSlug"`
	AppSlug      string `yaml:"appSlug"`
	Airgap       bool   `yaml:"airgap"`
}

// GetChannelRelease reads the embedded channel release object. If no channel release is found,
// returns nil and no error.
func parseChannelRelease(data []byte) (*ChannelRelease, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var release ChannelRelease
	if err := yaml.Unmarshal(data, &release); err != nil {
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
			return fmt.Errorf("failed to read file: %w", err)
		case header == nil:
			continue
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		content := bytes.NewBuffer(nil)
		if _, err := io.Copy(content, tr); err != nil {
			return fmt.Errorf("failed to copy file out of tar: %w", err)
		}

		switch {
		case bytes.Contains(content.Bytes(), []byte("apiVersion: kots.io/v1beta1")):
			if bytes.Contains(content.Bytes(), []byte("kind: Application")) {
				r.Application = content.Bytes()
			}

		case bytes.Contains(content.Bytes(), []byte("apiVersion: troubleshoot.sh/v1beta2")):
			if !bytes.Contains(content.Bytes(), []byte("kind: HostPreflight")) {
				break
			}
			if bytes.Contains(content.Bytes(), []byte("cluster.kurl.sh/v1beta1")) {
				break
			}
			hostPreflights, err := parseHostPreflights(content.Bytes())
			if err != nil {
				return fmt.Errorf("failed to parse host preflights: %w", err)
			}
			if hostPreflights != nil {
				if r.HostPreflights == nil {
					r.HostPreflights = &v1beta2.HostPreflightSpec{}
				}
				r.HostPreflights.Collectors = append(r.HostPreflights.Collectors, hostPreflights.Collectors...)
				r.HostPreflights.Analyzers = append(r.HostPreflights.Analyzers, hostPreflights.Analyzers...)
			}

		case bytes.Contains(content.Bytes(), []byte("apiVersion: embeddedcluster.replicated.com/v1beta1")):
			if !bytes.Contains(content.Bytes(), []byte("kind: Config")) {
				break
			}

			r.EmbeddedClusterConfig, err = parseEmbeddedClusterConfig(content.Bytes())
			if err != nil {
				return fmt.Errorf("failed to parse embedded cluster config: %w", err)
			}

		case bytes.Contains(content.Bytes(), []byte("apiVersion: velero.io/v1")):
			if bytes.Contains(content.Bytes(), []byte("kind: Backup")) {
				r.VeleroBackup, err = parseVeleroBackup(content.Bytes())
				if err != nil {
					return fmt.Errorf("failed to parse velero backup: %w", err)
				}
			} else if bytes.Contains(content.Bytes(), []byte("kind: Restore")) {
				r.VeleroRestore, err = parseVeleroRestore(content.Bytes())
				if err != nil {
					return fmt.Errorf("failed to parse velero restore: %w", err)
				}
			}

		case bytes.Contains(content.Bytes(), []byte("# channel release object")):
			r.ChannelRelease, err = parseChannelRelease(content.Bytes())
			if err != nil {
				return fmt.Errorf("failed to parse channel release: %w", err)
			}
		}
	}
}

// SetReleaseDataForTests should only be called from tests. It sets the release information based on the supplied data.
func SetReleaseDataForTests(data map[string][]byte) error {
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

	rd, err := newReleaseDataFrom(buf.Bytes())
	if err != nil {
		return fmt.Errorf("unable to set release data for tests: %w", err)
	}
	_releaseData = rd
	return nil
}
