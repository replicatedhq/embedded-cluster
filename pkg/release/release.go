package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/utils/pkg/embed"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"go.yaml.in/yaml/v3"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
	kyaml "sigs.k8s.io/yaml"
)

var (
	_releaseData *ReleaseData
)

// ReleaseData holds the parsed data from a Kots Release.
//
// Note / TODO: Custom resources (like HelmChart CRs) must be templated before they are parsed
// to avoid unmarshaling errors due to invalid schema that only becomes valid after templating.
type ReleaseData struct {
	data                  []byte
	Application           *kotsv1beta1.Application
	AppConfig             *kotsv1beta1.Config
	HostPreflights        *troubleshootv1beta2.HostPreflightSpec
	EmbeddedClusterConfig *ecv1beta1.Config
	ChannelRelease        *ChannelRelease
	VeleroBackup          *velerov1.Backup
	VeleroRestore         *velerov1.Restore
	HelmChartCRs          [][]byte
	HelmChartArchives     [][]byte
}

// GetReleaseData returns the release data.
func GetReleaseData() *ReleaseData {
	return _releaseData
}

// GetAppTitle returns the title from the kots application embedded as part of the
// release. If no application is found, returns an empty string.
func GetAppTitle() string {
	if _releaseData.Application == nil {
		return ""
	}
	return _releaseData.Application.Spec.Title
}

// GetHostPreflights returns a list of HostPreflight specs that are found in the
// binary. These are part of the embedded Kots Application Release.
func GetHostPreflights() *troubleshootv1beta2.HostPreflightSpec {
	return _releaseData.HostPreflights
}

// GetApplication reads and returns the kots application embedded as part of the
// release. If no application is found, returns nil and no error. This function does
// not unmarshal the application yaml.
func GetApplication() *kotsv1beta1.Application {
	return _releaseData.Application
}

// GetAppConfig reads and returns the kots app config embedded as part of the
// release. If no app config is found, returns nil and no error. This function does
// not unmarshal the app config yaml.
func GetAppConfig() *kotsv1beta1.Config {
	return _releaseData.AppConfig
}

// GetEmbeddedClusterConfig reads the embedded cluster config from the embedded Kots
// Application Release.
func GetEmbeddedClusterConfig() *ecv1beta1.Config {
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

// GetHelmChartCRs reads and returns the HelmChart custom resources embedded as part of the release.
// If no HelmChart CRs are found, returns an empty slice.
func GetHelmChartCRs() [][]byte {
	if _releaseData.HelmChartCRs == nil {
		return [][]byte{}
	}
	return _releaseData.HelmChartCRs
}

// GetHelmChartArchives reads and returns the Helm chart archives embedded as part of the release.
// If no chart archives are found, returns an empty slice.
func GetHelmChartArchives() [][]byte {
	if _releaseData.HelmChartArchives == nil {
		return [][]byte{}
	}
	return _releaseData.HelmChartArchives
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

func parseApplication(data []byte) (*kotsv1beta1.Application, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var app kotsv1beta1.Application
	if err := kyaml.Unmarshal(data, &app); err != nil {
		return nil, fmt.Errorf("unable to unmarshal application: %w", err)
	}
	return &app, nil
}

func parseAppConfig(data []byte) (*kotsv1beta1.Config, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var config kotsv1beta1.Config
	if err := kyaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unable to unmarshal app config: %w", err)
	}
	return &config, nil
}

func parseHostPreflights(data []byte) (*troubleshootv1beta2.HostPreflightSpec, error) {
	if len(data) == 0 {
		return nil, nil
	}
	return unserializeHostPreflightSpec(data)
}

// unserializeHostPreflightSpec unserializes a HostPreflightSpec from a raw slice of bytes.
func unserializeHostPreflightSpec(data []byte) (*troubleshootv1beta2.HostPreflightSpec, error) {
	scheme := kruntime.NewScheme()
	if err := troubleshootv1beta2.AddToScheme(scheme); err != nil {
		return nil, err
	}
	decoder := conversion.NewDecoder(scheme)
	var hpf troubleshootv1beta2.HostPreflight
	if err := decoder.DecodeInto(data, &hpf); err != nil {
		return nil, err
	}
	return &hpf.Spec, nil
}

func parseEmbeddedClusterConfig(data []byte) (*ecv1beta1.Config, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var cfg ecv1beta1.Config
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

func parseHelmChartCR(data []byte) (*kotsv1beta2.HelmChart, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var helmChart kotsv1beta2.HelmChart
	if err := kyaml.Unmarshal(data, &helmChart); err != nil {
		return nil, fmt.Errorf("unable to unmarshal helm chart CR: %w", err)
	}
	return &helmChart, nil
}

// ChannelRelease contains information about a specific app release inside a channel.
type ChannelRelease struct {
	VersionLabel    string  `yaml:"versionLabel"`
	ChannelID       string  `yaml:"channelID"`
	ChannelSequence int64   `yaml:"channelSequence"`
	ChannelSlug     string  `yaml:"channelSlug"`
	AppSlug         string  `yaml:"appSlug"`
	Airgap          bool    `yaml:"airgap"`
	DefaultDomains  Domains `yaml:"defaultDomains"`
}

type Domains struct {
	ReplicatedAppDomain      string `yaml:"replicatedAppDomain,omitempty"`
	ProxyRegistryDomain      string `yaml:"proxyRegistryDomain,omitempty"`
	ReplicatedRegistryDomain string `yaml:"replicatedRegistryDomain,omitempty"`
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

		// we process special files without splitting YAML documents as either they are not yaml or
		// they are the release data itself which is identified by a comment at the beginning of
		// the file
		if err := r.processDocument(content.Bytes(), header.Name); err != nil {
			return err
		}

		if !strings.HasPrefix(header.Name, ".") && (strings.HasSuffix(header.Name, ".yaml") || strings.HasSuffix(header.Name, ".yml")) {
			// Split multi-document YAML files
			documents, err := splitYAMLDocuments(content.Bytes())
			if err != nil {
				// log only and do not fail here to preserve the previous behavior
				log.Printf("Failed to parse YAML document from release data %s: %v", header.Name, err)
			} else {
				// Process each document
				for _, doc := range documents {
					if err := r.processYAMLDocument(doc, header.Name); err != nil {
						return err
					}
				}
				// no need to process the document further
				continue
			}
		}

		// for backward compatibility, process files again as YAML documents that failed to parse
		// or do not have the yaml extension
		if err := r.processYAMLDocument(content.Bytes(), header.Name); err != nil {
			return err
		}
	}
}

// processDocument processes a single non-YAML document and updates the ReleaseData accordingly.
func (r *ReleaseData) processDocument(content []byte, headerName string) error {
	var err error

	switch {
	case bytes.Contains(content, []byte("# channel release object")):
		r.ChannelRelease, err = parseChannelRelease(content)
		if err != nil {
			return fmt.Errorf("failed to parse channel release: %w", err)
		}

	case strings.HasSuffix(headerName, ".tgz"):
		// Skip system files (like macOS ._* files)
		if isSystemFile(headerName) {
			break
		}

		// This is a chart archive (.tgz file)
		if r.HelmChartArchives == nil {
			r.HelmChartArchives = [][]byte{}
		}
		r.HelmChartArchives = append(r.HelmChartArchives, content)
	}

	return nil
}

// processYAMLDocument processes a single YAML document and updates the ReleaseData accordingly.
func (r *ReleaseData) processYAMLDocument(content []byte, headerName string) error {
	var err error

	switch {
	case bytes.Contains(content, []byte("apiVersion: kots.io/v1beta1")):
		if bytes.Contains(content, []byte("kind: Application")) {
			parsed, err := parseApplication(content)
			if err != nil {
				return fmt.Errorf("failed to parse application: %w", err)
			}
			r.Application = parsed
		} else if bytes.Contains(content, []byte("kind: Config")) {
			parsed, err := parseAppConfig(content)
			if err != nil {
				return fmt.Errorf("failed to parse app config: %w", err)
			}
			r.AppConfig = parsed
		}

	case bytes.Contains(content, []byte("apiVersion: troubleshoot.sh/v1beta2")):
		if !bytes.Contains(content, []byte("kind: HostPreflight")) {
			break
		}
		if bytes.Contains(content, []byte("cluster.kurl.sh/v1beta1")) {
			break
		}
		hostPreflights, err := parseHostPreflights(content)
		if err != nil {
			return fmt.Errorf("failed to parse host preflights: %w", err)
		}
		if hostPreflights != nil {
			if r.HostPreflights == nil {
				r.HostPreflights = &troubleshootv1beta2.HostPreflightSpec{}
			}
			r.HostPreflights.Collectors = append(r.HostPreflights.Collectors, hostPreflights.Collectors...)
			r.HostPreflights.Analyzers = append(r.HostPreflights.Analyzers, hostPreflights.Analyzers...)
		}

	case bytes.Contains(content, []byte("apiVersion: embeddedcluster.replicated.com/v1beta1")):
		if !bytes.Contains(content, []byte("kind: Config")) {
			break
		}

		r.EmbeddedClusterConfig, err = parseEmbeddedClusterConfig(content)
		if err != nil {
			return fmt.Errorf("failed to parse embedded cluster config: %w", err)
		}

	case bytes.Contains(content, []byte("apiVersion: velero.io/v1")):
		if bytes.Contains(content, []byte("kind: Backup")) {
			r.VeleroBackup, err = parseVeleroBackup(content)
			if err != nil {
				return fmt.Errorf("failed to parse velero backup: %w", err)
			}
		} else if bytes.Contains(content, []byte("kind: Restore")) {
			r.VeleroRestore, err = parseVeleroRestore(content)
			if err != nil {
				return fmt.Errorf("failed to parse velero restore: %w", err)
			}
		}

	case bytes.Contains(content, []byte("apiVersion: kots.io/v1beta2")):
		if bytes.Contains(content, []byte("kind: HelmChart")) {
			if r.HelmChartCRs == nil {
				r.HelmChartCRs = [][]byte{}
			}
			r.HelmChartCRs = append(r.HelmChartCRs, content)
		}
	}

	return nil
}

// splitYAMLDocuments splits a multi-document YAML file into individual documents.
func splitYAMLDocuments(data []byte) ([][]byte, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))

	var res [][]byte
	for {
		var value interface{}
		err := dec.Decode(&value)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		valueBytes, err := yaml.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		res = append(res, valueBytes)
	}
	return res, nil
}

// isSystemFile returns true if the filename represents a system file that should be ignored
func isSystemFile(filename string) bool {
	basename := filepath.Base(filename)

	// macOS AppleDouble files (resource forks and extended attributes)
	if strings.HasPrefix(basename, "._") {
		return true
	}

	// macOS Finder metadata
	if basename == ".DS_Store" {
		return true
	}

	// Windows Thumbs.db
	if basename == "Thumbs.db" {
		return true
	}

	return false
}

// SetReleaseDataForTests should only be called from tests. It sets the release information based on the supplied data.
func SetReleaseDataForTests(data map[string][]byte) error {
	buf := bytes.NewBuffer([]byte{})
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	for name, content := range data {
		err := tw.WriteHeader(&tar.Header{
			Name: name,
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
