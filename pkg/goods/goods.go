// Package goods handles embedded static assets. Things like writing them
// down to disk, return them as a parsed list, etc.
package goods

import (
	"embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/replicatedhq/helmvm/pkg/defaults"
)

//go:embed bins/*
var binfs embed.FS

//go:embed images/*
var imgfs embed.FS

//go:embed web/*
var webfs embed.FS

// Materialize writes to disk embed assets.
func Materialize() error {
	entries, err := binfs.ReadDir("bins/k0sctl")
	if err != nil {
		return fmt.Errorf("unable to read bins dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcpath := fmt.Sprintf("bins/k0sctl/%s", entry.Name())
		srcfile, err := binfs.ReadFile(srcpath)
		if err != nil {
			return fmt.Errorf("unable to read asset: %w", err)
		}
		dstpath := fmt.Sprintf("%s/%s", defaults.K0sctlBinsSubDir(), entry.Name())
		if err := os.WriteFile(dstpath, srcfile, 0755); err != nil {
			return fmt.Errorf("unable to write file: %w", err)
		}
	}
	entries, err = binfs.ReadDir("bins/helmvm")
	if err != nil {
		return fmt.Errorf("unable to read helmvm bins dir: %w", err)
	}
	suffix := fmt.Sprintf("-%s-%s", runtime.GOOS, runtime.GOARCH)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			// we always materialize 'preflight' binary because
			// we run it remotely in the configured cluster nodes.
			if entry.Name() != "preflight" {
				continue
			}
		}
		srcpath := fmt.Sprintf("bins/helmvm/%s", entry.Name())
		srcfile, err := binfs.ReadFile(srcpath)
		if err != nil {
			return fmt.Errorf("unable to read asset: %w", err)
		}
		fname := strings.TrimSuffix(entry.Name(), suffix)
		dstpath := fmt.Sprintf("%s/%s", defaults.HelmVMBinsSubDir(), fname)
		if err := os.WriteFile(dstpath, srcfile, 0755); err != nil {
			return fmt.Errorf("unable to write file: %w", err)
		}
	}
	return nil
}

// WebFile returns the content of a file from within the goods/web directory.
func WebFile(name string) ([]byte, error) {
	return webfs.ReadFile(fmt.Sprintf("web/%s", name))
}

// ListImages returns all the images found on embed images/list.txt file.
func ListImages() ([]string, error) {
	content, err := imgfs.ReadFile("images/list.txt")
	if err != nil {
		return nil, fmt.Errorf("unable to read list.txt: %w", err)
	}
	contentstr := string(content)
	lines := strings.Split(contentstr, "\n")
	var images []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			images = append(images, line)
		}
	}
	return images, nil
}

// DownloadImagesBundle starts a download and returns a reader from where the
// bundle can be read. To close the resturn value is a caller responsibility.
func DownloadImagesBundle(version string) (io.ReadCloser, error) {
	baseurl := "https://github.com/k0sproject/k0s/releases"
	urlpath := "download/%[1]s/k0s-airgap-bundle-%[1]s-amd64"
	urltpl := fmt.Sprintf("%s/%s", baseurl, urlpath)
	bundleURL := fmt.Sprintf(urltpl, version)
	response, err := http.Get(bundleURL)
	if err != nil {
		return nil, fmt.Errorf("unable to download bundle: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to download bundle: %s", response.Status)
	}
	return response.Body, nil
}
