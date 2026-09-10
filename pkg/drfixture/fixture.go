package drfixture

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxInspectionDepth = 6

const Schema = "embedded-cluster-dr-fixture/v1"

type Manifest struct {
	Schema        string `json:"schema"`
	CreatedAt     string `json:"createdAt"`
	ECVersion     string `json:"ecVersion"`
	K0sVersion    string `json:"k0sVersion"`
	Application   string `json:"applicationVersion"`
	BundleSHA256  string `json:"bundleSHA256"`
	S3Region      string `json:"s3Region"`
	S3Bucket      string `json:"s3Bucket"`
	S3Prefix      string `json:"s3Prefix"`
	S3AccessKey   string `json:"s3AccessKey"`
	S3SecretKey   string `json:"s3SecretKey"`
	Payload       string `json:"payload"`
	PayloadSHA256 string `json:"payloadSHA256"`
}

func WriteManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal fixture manifest: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write fixture manifest: %w", err)
	}
	return nil
}

func Verify(payloadPath, manifestPath string) (*Manifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read fixture manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode fixture manifest: %w", err)
	}
	if manifest.Schema != Schema || manifest.CreatedAt == "" || manifest.ECVersion == "" ||
		manifest.K0sVersion == "" || manifest.Application == "" || manifest.BundleSHA256 == "" || manifest.S3Region == "" ||
		manifest.S3Bucket == "" || manifest.S3Prefix == "" || manifest.S3AccessKey == "" ||
		manifest.S3SecretKey == "" || manifest.PayloadSHA256 == "" {
		return nil, fmt.Errorf("fixture manifest is incomplete or has unsupported schema %q", manifest.Schema)
	}
	if manifest.Payload != filepath.Base(payloadPath) {
		return nil, fmt.Errorf("fixture manifest payload %q does not match %q", manifest.Payload, filepath.Base(payloadPath))
	}
	digest, err := FileSHA256(payloadPath)
	if err != nil {
		return nil, err
	}
	if digest != manifest.PayloadSHA256 {
		return nil, fmt.Errorf("fixture payload digest mismatch: got %s, want %s", digest, manifest.PayloadSHA256)
	}
	if err := verifyArchive(payloadPath); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func verifyArchive(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open fixture archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open fixture gzip stream: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	files := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read fixture archive: %w", err)
		}
		clean := filepath.ToSlash(filepath.Clean(header.Name))
		if clean != "data" && !strings.HasPrefix(clean, "data/") {
			return fmt.Errorf("fixture archive entry escapes data directory: %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
		case tar.TypeReg, tar.TypeRegA:
			files++
		default:
			return fmt.Errorf("fixture archive contains unsupported entry type for %q", header.Name)
		}
	}
	if files == 0 {
		return fmt.Errorf("fixture archive contains no MinIO objects")
	}
	return nil
}

func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// RejectValues recursively inspects the fixture and nested gzip/tar payloads
// for exact credential values. Labels are reported, but values never are.
func RejectValues(payloadPath string, forbidden map[string]string) error {
	patterns := make(map[string][]byte, len(forbidden))
	for label, value := range forbidden {
		if value == "" {
			return fmt.Errorf("forbidden value %q is empty", label)
		}
		patterns[label] = []byte(value)
	}
	tempDir, err := os.MkdirTemp("", "dr-fixture-inspect-*")
	if err != nil {
		return fmt.Errorf("create fixture inspection directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	return inspectFile(payloadPath, filepath.Base(payloadPath), patterns, tempDir, 0)
}

func inspectFile(path, logicalPath string, patterns map[string][]byte, tempDir string, depth int) error {
	if depth > maxInspectionDepth {
		return fmt.Errorf("fixture archive nesting exceeds %d at %s", maxInspectionDepth, logicalPath)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", logicalPath, err)
	}
	if err := rejectReaderValues(f, logicalPath, patterns); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return err
	}
	reader := bufio.NewReader(f)
	header, _ := reader.Peek(512)
	isGzip := len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b
	isTar := len(header) >= 262 && bytes.Equal(header[257:262], []byte("ustar"))
	if !isGzip && !isTar {
		return f.Close()
	}

	if isGzip {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			f.Close()
			return fmt.Errorf("open nested gzip %s: %w", logicalPath, err)
		}
		expanded, err := os.CreateTemp(tempDir, "expanded-*")
		if err != nil {
			gz.Close()
			f.Close()
			return fmt.Errorf("create expanded fixture file: %w", err)
		}
		_, err = io.Copy(expanded, gz)
		closeErr := expanded.Close()
		gzErr := gz.Close()
		f.Close()
		if err != nil {
			return fmt.Errorf("expand nested gzip %s: %w", logicalPath, err)
		}
		if closeErr != nil {
			return closeErr
		}
		if gzErr != nil {
			return fmt.Errorf("close nested gzip %s: %w", logicalPath, gzErr)
		}
		defer os.Remove(expanded.Name())
		return inspectFile(expanded.Name(), logicalPath+"<gzip>", patterns, tempDir, depth+1)
	}

	err = inspectTar(reader, logicalPath, patterns, tempDir, depth)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func inspectTar(reader io.Reader, logicalPath string, patterns map[string][]byte, tempDir string, depth int) error {
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect nested tar %s: %w", logicalPath, err)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		entry, err := os.CreateTemp(tempDir, "entry-*")
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(entry, tr)
		closeErr := entry.Close()
		if copyErr != nil {
			os.Remove(entry.Name())
			return copyErr
		}
		if closeErr != nil {
			os.Remove(entry.Name())
			return closeErr
		}
		entryLogicalPath := logicalPath + ":" + header.Name
		err = inspectFile(entry.Name(), entryLogicalPath, patterns, tempDir, depth+1)
		os.Remove(entry.Name())
		if err != nil {
			return err
		}
	}
}

func rejectReaderValues(reader io.Reader, logicalPath string, patterns map[string][]byte) error {
	maxPattern := 0
	for _, pattern := range patterns {
		if len(pattern) > maxPattern {
			maxPattern = len(pattern)
		}
	}
	if maxPattern == 0 {
		return nil
	}
	buffer := make([]byte, 64*1024+maxPattern-1)
	retained := 0
	for {
		n, err := reader.Read(buffer[retained:])
		data := buffer[:retained+n]
		for label, pattern := range patterns {
			if bytes.Contains(data, pattern) {
				return fmt.Errorf("fixture contains forbidden value %q in %s", label, logicalPath)
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect %s: %w", logicalPath, err)
		}
		retained = min(len(data), maxPattern-1)
		copy(buffer[:retained], data[len(data)-retained:])
	}
}
