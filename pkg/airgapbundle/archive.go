package airgapbundle

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"sigs.k8s.io/yaml"
)

type BundleInput struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type BundleManifest struct {
	Schema      string            `json:"schema"`
	Application BundleInput       `json:"application"`
	ECFiles     map[string]string `json:"ecFiles"`
}

// Augment overlays a fully constructed EC v2 directory on an application
// airgap bundle. ecDir must follow the production embedded-cluster/ layout.
func Augment(applicationBundle, ecDir, versionLabel, output string) error {
	if versionLabel == "" {
		return fmt.Errorf("version label is required")
	}
	tmp, err := os.MkdirTemp("", "ec-airgap-bundle-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := extractTarGz(applicationBundle, tmp); err != nil {
		return fmt.Errorf("extract application bundle: %w", err)
	}
	for _, required := range []string{"embedded-cluster-amd64", "version-metadata.json", "images-amd64.tar", "charts.tar.gz"} {
		if info, err := os.Stat(filepath.Join(ecDir, required)); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("required EC bundle component %s is missing", required)
		}
	}
	destEC := filepath.Join(tmp, "embedded-cluster")
	if err := copyTree(ecDir, destEC); err != nil {
		return err
	}
	if err := updateAirgapMetadata(tmp, destEC, versionLabel); err != nil {
		return err
	}
	manifest := BundleManifest{Schema: "v1", ECFiles: map[string]string{}}
	manifest.Application = BundleInput{Path: filepath.Base(applicationBundle), SHA256: fileSHA256(applicationBundle)}
	if err := filepath.WalkDir(destEC, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(tmp, path)
		if err != nil {
			return err
		}
		manifest.ECFiles[filepath.ToSlash(rel)] = fileSHA256(path)
		return nil
	}); err != nil {
		return err
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(output+".manifest.json", b, 0o644); err != nil {
		return err
	}
	return WriteDeterministicTarGz(tmp, output)
}

func updateAirgapMetadata(workspace, ecDir, versionLabel string) error {
	path := filepath.Join(workspace, "airgap.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read application airgap metadata: %w", err)
	}
	var metadata kotsv1beta1.Airgap
	if err := yaml.Unmarshal(b, &metadata); err != nil {
		return fmt.Errorf("decode application airgap metadata: %w", err)
	}
	metadata.Spec.VersionLabel = versionLabel
	a := &kotsv1beta1.EmbeddedClusterArtifacts{
		BinaryAmd64: "embedded-cluster/embedded-cluster-amd64", Metadata: "embedded-cluster/version-metadata.json",
		Charts: "embedded-cluster/charts.tar.gz", ImagesAmd64: "embedded-cluster/images-amd64.tar",
		AdditionalArtifacts: map[string]string{},
	}
	for _, name := range []string{"kots", "operator", "manager"} {
		if _, err := os.Stat(filepath.Join(ecDir, "artifacts", name+".tar.gz")); err == nil {
			a.AdditionalArtifacts[name] = "embedded-cluster/artifacts/" + name + ".tar.gz"
		}
	}
	if len(a.AdditionalArtifacts) == 0 {
		a.AdditionalArtifacts = nil
	}
	if info, err := os.Stat(filepath.Join(ecDir, "registry")); err == nil && info.IsDir() {
		a.Registry.Dir = "embedded-cluster/registry"
	}
	metadata.Spec.EmbeddedClusterArtifacts = a
	// Marshal until the self-referential uncompressed size stabilizes.
	for range 4 {
		encoded, err := yaml.Marshal(&metadata)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			return err
		}
		size, err := treeSize(workspace)
		if err != nil {
			return err
		}
		if metadata.Spec.UncompressedSize == size {
			return nil
		}
		metadata.Spec.UncompressedSize = size
	}
	encoded, err := yaml.Marshal(&metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o644)
}

func treeSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

// WrapRelease creates the customer-facing v2 response archive.
func WrapRelease(appSlug, installer, license, airgapBundle, output string) error {
	if appSlug == "" || filepath.Base(appSlug) != appSlug {
		return fmt.Errorf("invalid application slug %q", appSlug)
	}
	tmp, err := os.MkdirTemp("", "ec-airgap-release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := copyFile(installer, filepath.Join(tmp, appSlug), 0o755); err != nil {
		return err
	}
	if err := copyFile(license, filepath.Join(tmp, "license.yaml"), 0o644); err != nil {
		return err
	}
	if err := copyFile(airgapBundle, filepath.Join(tmp, appSlug+".airgap"), 0o644); err != nil {
		return err
	}
	return WriteDeterministicTarGz(tmp, output)
}

func copyTree(source, dest string) error {
	return filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func extractTarGz(path, dest string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(h.Name))
		if name == "." {
			continue
		}
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe archive path %q", h.Name)
		}
		target := filepath.Join(dest, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode)&0o777)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("unsupported archive entry %q (type %d)", h.Name, h.Typeflag)
		}
	}
}

func copyFile(source, dest string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
