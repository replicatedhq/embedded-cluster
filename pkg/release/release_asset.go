package release

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	mtx         sync.Mutex
	downloading = make(map[string]bool)
)

// releaseAsset represents an embedded release binary as stored in the Amazon S3 bucket.
// The release is store remotely as a tgz file so this struct provides tooling to read
// the release (stored as a tgz file) directly as the actual binary inside the tgz.
type releaseAsset struct {
	s3      *s3.GetObjectOutput
	gz      *gzip.Reader
	tar     *tar.Reader
	local   *os.File
	version string
	failure bool
}

// Read calls Read in the tar reader. After calling ff() the reader is positioned at
// the beginning of the embedded-cluster binary. This functions also pipes all read
// data to the local file if it is set.
func (r *releaseAsset) Read(p []byte) (int, error) {
	read, err := r.tar.Read(p)
	if r.local == nil {
		return read, err
	}
	if wrote, err := r.local.Write(p[:read]); err != nil {
		r.failure = true
		return wrote, err
	}
	r.failure = err != nil && err != io.EOF
	return read, err
}

// Close makes sure we close all the readers.
func (r *releaseAsset) Close() error {
	if r.local != nil {
		r.local.Close()
	}
	mtx.Lock()
	if r.failure {
		os.Remove(r.local.Name())
	}
	delete(downloading, r.version)
	mtx.Unlock()
	r.gz.Close()
	r.s3.Body.Close()
	return nil
}

// ff fast-forward the tar reader until we find the embedded-cluster binary and then
// stops. This is useful to make sure the next time someone calls Read() we are at
// the beginning of the embedded-cluster binary.
func (r *releaseAsset) ff() error {
	for {
		if head, err := r.tar.Next(); err != nil {
			return fmt.Errorf("unable to read tar header: %w", err)
		} else if head.Typeflag != tar.TypeReg {
			continue
		} else if head.Name != "embedded-cluster" {
			continue
		}
		return nil
	}
}

// setupPipe makes sure that we have a file on disk to where we write the release
// binary. Everytime one calls Read on this struct the data is written to this file
// while also being returned to the caller.
func (r *releaseAsset) setupPipe() error {
	var err error
	path := path.Join(localReleases, r.version)
	r.local, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("unable to create local release file: %w", err)
	}
	return nil
}

// newReleaseAsset returns a new releaseAsset struct that can be used to read the
// embedded-cluster binary from the given s3 object.
func newReleaseAsset(obj *s3.GetObjectOutput, version string) (*releaseAsset, error) {
	gzReader, err := gzip.NewReader(obj.Body)
	if err != nil {
		obj.Body.Close()
		return nil, fmt.Errorf("unable to create gzip reader: %w", err)
	}
	tarReader := tar.NewReader(gzReader)
	asset := &releaseAsset{s3: obj, gz: gzReader, tar: tarReader, version: version}
	if err := asset.ff(); err != nil {
		asset.Close()
		return nil, fmt.Errorf("unable to fast-forward tar reader: %w", err)
	}
	mtx.Lock()
	defer mtx.Unlock()
	if _, ok := downloading[version]; ok {
		return asset, nil
	}
	downloading[version] = true
	if err := asset.setupPipe(); err != nil {
		asset.Close()
		return nil, fmt.Errorf("unable to setup pipe: %w", err)
	}
	return asset, nil
}
