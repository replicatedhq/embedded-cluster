package airgapbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// ApplicationIdentity is the immutable identity of an application-only E2E release.
type ApplicationIdentity struct {
	Role    string `json:"role"`
	Digest  string `json:"digest"`
	Version string `json:"version"`
}

// ApplicationDigest hashes normalized paths and contents. The role is part of
// the version identity because multiple channel releases may use identical sources.
func ApplicationDigest(role string, roots ...string) (ApplicationIdentity, error) {
	if role == "" {
		return ApplicationIdentity{}, fmt.Errorf("role is required")
	}
	type input struct{ name, path string }
	var inputs []input
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			return ApplicationIdentity{}, err
		}
		base := filepath.Base(filepath.Clean(root))
		if !info.IsDir() {
			inputs = append(inputs, input{base, root})
			continue
		}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			inputs = append(inputs, input{filepath.ToSlash(filepath.Join(base, rel)), path})
			return nil
		})
		if err != nil {
			return ApplicationIdentity{}, err
		}
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].name < inputs[j].name })
	h := sha256.New()
	for _, in := range inputs {
		data, err := os.ReadFile(in.path)
		if err != nil {
			return ApplicationIdentity{}, err
		}
		_, _ = io.WriteString(h, in.name+"\x00")
		_, _ = h.Write(data)
		_, _ = h.Write([]byte{0})
	}
	digest := hex.EncodeToString(h.Sum(nil))
	return ApplicationIdentity{Role: role, Digest: digest, Version: fmt.Sprintf("airgap-e2e-%s-sha%s", role, digest[:16])}, nil
}

// SeparateConfig copies a release spec while removing Embedded Cluster Config
// documents. Removed documents are written to configPath for local augmentation.
func SeparateConfig(sourceDir, applicationDir, configPath string) error {
	if err := os.MkdirAll(applicationDir, 0o755); err != nil {
		return err
	}
	var configs []*yaml.Node
	err := filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(applicationDir, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" {
			return os.WriteFile(dest, data, 0o644)
		}
		dec := yaml.NewDecoder(bytes.NewReader(data))
		var kept []*yaml.Node
		for {
			var doc yaml.Node
			if err := dec.Decode(&doc); err != nil {
				if err == io.EOF {
					break
				}
				return fmt.Errorf("decode %s: %w", path, err)
			}
			if len(doc.Content) == 0 {
				continue
			}
			if yamlScalar(&doc, "apiVersion") == "embeddedcluster.replicated.com/v1beta1" && yamlScalar(&doc, "kind") == "Config" {
				configs = append(configs, &doc)
			} else {
				kept = append(kept, &doc)
			}
		}
		if len(kept) == 0 {
			return nil
		}
		return encodeYAMLDocuments(dest, kept)
	})
	if err != nil {
		return err
	}
	if len(configs) != 1 {
		return fmt.Errorf("expected exactly one Embedded Cluster Config, found %d", len(configs))
	}
	return encodeYAMLDocuments(configPath, configs)
}

func yamlScalar(doc *yaml.Node, key string) string {
	if len(doc.Content) == 0 || len(doc.Content[0].Content) == 0 {
		return ""
	}
	n := doc.Content[0]
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1].Value
		}
	}
	return ""
}

func encodeYAMLDocuments(path string, docs []*yaml.Node) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	for _, doc := range docs {
		if err := enc.Encode(doc); err != nil {
			return err
		}
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// WriteDeterministicTarGz creates a byte-reproducible archive of root.
func WriteDeterministicTarGz(root, output string) error {
	return writeDeterministicTar(root, output, true)
}

func WriteDeterministicTar(root, output string) error {
	return writeDeterministicTar(root, output, false)
}

func writeDeterministicTar(root, output string, compressed bool) error {
	var names []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		names = append(names, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(names)
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()
	var writer io.Writer = f
	var gz *gzip.Writer
	if compressed {
		gz = gzip.NewWriter(f)
		gz.Header.ModTime = time.Unix(0, 0)
		gz.Header.OS = 255
		writer = gz
	}
	tw := tar.NewWriter(writer)
	for _, name := range names {
		path := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = strings.TrimSuffix(name, "/")
		if info.IsDir() {
			hdr.Name += "/"
		}
		hdr.ModTime, hdr.AccessTime, hdr.ChangeTime = time.Unix(0, 0), time.Time{}, time.Time{}
		hdr.Uid, hdr.Gid, hdr.Uname, hdr.Gname = 0, 0, "", ""
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			in, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, in)
			closeErr := in.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if gz != nil {
		if err := gz.Close(); err != nil {
			return err
		}
	}
	return f.Close()
}
