package manager

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadBinaryOnline(t *testing.T) {
	// Create a temporary directory for our test files
	tmpDir, err := os.MkdirTemp("", "binary-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create the test content
	testContent := []byte("TESTING")
	testFile := filepath.Join(tmpDir, "manager")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create a tar.gz file containing our test file
	tarGzPath := filepath.Join(tmpDir, "manager.tar.gz")
	if err := createTestTarGz(tarGzPath, testFile, "manager"); err != nil {
		t.Fatalf("Failed to create test tar.gz: %v", err)
	}

	// Start a test server that serves our tar.gz file
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify basic auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "testlicense" || password != "testlicense" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Serve the tar.gz file
		http.ServeFile(w, r, tarGzPath)
	}))
	defer server.Close()

	// Create destination path for our binary
	dstPath := filepath.Join(tmpDir, "downloaded-manager")

	// Test the download function
	err = DownloadBinaryOnline(
		context.Background(),
		dstPath,
		"testlicense",
		server.URL,
		"testversion",
	)
	if err != nil {
		t.Fatalf("DownloadBinaryOnline failed: %v", err)
	}

	// Verify the downloaded content
	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(content) != "TESTING" {
		t.Errorf("Expected content 'TESTING', got '%s'", string(content))
	}
}

func createTestTarGz(tarGzPath, srcPath, tarPath string) error {
	tarGzFile, err := os.Create(tarGzPath)
	if err != nil {
		return err
	}
	defer tarGzFile.Close()

	gzWriter := gzip.NewWriter(tarGzFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    tarPath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	if _, err := io.Copy(tarWriter, file); err != nil {
		return err
	}

	return nil
}
