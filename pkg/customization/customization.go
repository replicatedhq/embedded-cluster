package customization

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"debug/elf"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"sigs.k8s.io/yaml"

	"github.com/replicatedhq/helmvm/pkg/preflights"
)

// ParsedSection holds the parsed section from the binary. We only care about the
// application object, whatever HostPreflight we can find, and the app License.
type ParsedSection struct {
	Application           []byte
	HostPreflights        [][]byte
	License               []byte
	EmbeddedClusterConfig []byte
}

// AdminConsole is a struct that contains the actions to create and update the admin
// console customization found inside the binary. This is necessary for backwards
// compatibility with older versions of helmvm.
type AdminConsole struct{}

// ExtractCustomization will extract the customization from the binary if it exists.
// The customization is expected to be found in the sec_bundle section of the binary.
func (a AdminConsole) ExtractCustomization() (*ParsedSection, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	fpbin, err := elf.Open(exe)
	if err != nil {
		return nil, err
	}
	defer fpbin.Close()
	section := fpbin.Section("sec_bundle")
	if section == nil {
		return nil, nil
	}
	return a.processSection(section)
}

// processSection searches the provided elf section for a gzip compressed tar archive.
// If it finds one, it will extract the contents and return the kots.io Application
// and any HostPrefligth objects as a byte slice.
func (a AdminConsole) processSection(section *elf.Section) (*ParsedSection, error) {
	gzr, err := gzip.NewReader(section.Open())
	if err != nil {
		return nil, err
	}
	defer gzr.Close()
	result := &ParsedSection{}
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return result, nil
		case err != nil:
			return nil, fmt.Errorf("unable to read tgz file: %w", err)
		case header == nil:
			continue
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		content := bytes.NewBuffer(nil)
		if _, err := io.Copy(content, tr); err != nil {
			return nil, fmt.Errorf("unable to copy file out of tar: %w", err)
		}
		if bytes.Contains(content.Bytes(), []byte("apiVersion: kots.io/v1beta1")) {
			if bytes.Contains(content.Bytes(), []byte("kind: Application")) {
				result.Application = content.Bytes()
			}
			if bytes.Contains(content.Bytes(), []byte("kind: License")) {
				result.License = content.Bytes()
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
			result.HostPreflights = append(result.HostPreflights, content.Bytes())
		}
		if bytes.Contains(content.Bytes(), []byte("apiVersion: embeddedcluster.replicated.com/v1beta1")) {
			if bytes.Contains(content.Bytes(), []byte("kind: Config")) {
				continue
			}
			result.EmbeddedClusterConfig = content.Bytes()
		}
	}
}

// HostPreflights returns a list of HostPreflight specs that are found in the binary.
// These are part of the embedded Kots Application Release.
func (a AdminConsole) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	if runtime.GOOS != "linux" {
		return &v1beta2.HostPreflightSpec{}, nil
	}
	section, err := a.ExtractCustomization()
	if err != nil {
		return nil, err
	} else if section == nil {
		return &v1beta2.HostPreflightSpec{}, nil
	}
	all := &v1beta2.HostPreflightSpec{}
	for _, serialized := range section.HostPreflights {
		spec, err := preflights.UnserializeSpec(serialized)
		if err != nil {
			return nil, fmt.Errorf("unable to unserialize preflight spec: %w", err)
		}
		all.Collectors = append(all.Collectors, spec.Collectors...)
		all.Analyzers = append(all.Analyzers, spec.Analyzers...)
	}
	return all, nil
}

// License reads the kots license from the embedded Kots Application Release. If no license is found,
// returns nil and no error.
func (a AdminConsole) License() (*v1beta1.License, error) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	section, err := a.ExtractCustomization()
	if err != nil {
		return nil, err
	} else if section == nil {
		return nil, nil
	}
	var license v1beta1.License
	if err := yaml.Unmarshal(section.License, &license); err != nil {
		return nil, fmt.Errorf("failed to unmarshal license: %w", err)
	}
	return &license, nil
}

// Application reads and returns the kots application embedded as part of the release. If no
// application is found, returns nil and no error. This function does not unmarshal the
// application yaml.
func (a AdminConsole) Application() ([]byte, error) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	section, err := a.ExtractCustomization()
	if err != nil {
		return nil, err
	} else if section == nil {
		return nil, nil
	}
	return section.Application, nil
}

// EmbeddedClusterConfig reads the embedded cluster config from the embedded Kots Application Release.
func (a AdminConsole) EmbeddedClusterConfig() ([]byte, error) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	section, err := a.ExtractCustomization()
	if err != nil {
		return nil, err
	} else if section == nil {
		return nil, nil
	}
	return section.EmbeddedClusterConfig, nil
}
