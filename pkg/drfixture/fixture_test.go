package drfixture

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestRejectValuesFindsValueAcrossReadBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload")
	data := append(make([]byte, 64*1024-3), []byte("fixture-secret")...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RejectValues(path, map[string]string{"test credential": "fixture-secret"}); err == nil {
		t.Fatal("expected forbidden value error")
	}
}

func TestRejectValuesFindsValueInNestedTarGzip(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "inner.tar.gz")
	writeTarGzip(t, inner, "backup/credentials", []byte("prefix-fixture-secret-suffix"))
	innerData, err := os.ReadFile(inner)
	if err != nil {
		t.Fatal(err)
	}
	outer := filepath.Join(dir, "outer.tar.gz")
	writeTarGzip(t, outer, "data/bucket/backup.tar.gz", innerData)
	if err := RejectValues(outer, map[string]string{"test credential": "fixture-secret"}); err == nil {
		t.Fatal("expected forbidden value error")
	}
}

func TestRejectValuesAcceptsCleanNestedArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.tar.gz")
	writeTarGzip(t, path, "data/bucket/backup", []byte("fixture contents"))
	if err := RejectValues(path, map[string]string{"test credential": "absent-secret"}); err != nil {
		t.Fatalf("inspect clean fixture: %v", err)
	}
}

func writeTarGzip(t *testing.T, path, name string, contents []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(contents)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
