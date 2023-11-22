// Package release manages the logic of building customized embedded cluster binaries. Such
// binaries include a KOTS release.
package release

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// SizeReadCloser is an abstraction for an object from where we can read from, close, and has
// a size.
type SizeReadCloser interface {
	Size() int64
	io.ReadCloser
}

// RawSource is an abstraction that exists for test purposes. It is implemented by the Source
// struct in source.go. Release source is an entity capable of providing us with path to a raw
// (compiled from source) embedded cluster binary.
type RawSource interface {
	binaryFor(ctx context.Context, version string) (io.ReadCloser, error)
}

// Builder builds a new embedded cluster binary.
type Builder struct {
	src RawSource
}

// Build builds a new embedded cluster binary, returns a reader to the resulting tar.gz file.
func (b *Builder) Build(ctx context.Context, req *BuildRequest) (SizeReadCloser, error) {
	binfp, err := b.src.binaryFor(ctx, req.EmbeddedClusterVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to get binary: %v", err)
	}
	defer binfp.Close()
	bctx, err := newBuildContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("unable to create build context: %v", err)
	}
	if err := bctx.withSourceBinary(binfp); err != nil {
		bctx.Close()
		return nil, fmt.Errorf("unable to write binary: %v", err)
	}
	if err := b.embed(bctx); err != nil {
		bctx.Close()
		return nil, fmt.Errorf("unable to embed: %v", err)
	}
	if err := b.compress(bctx); err != nil {
		bctx.Close()
		return nil, fmt.Errorf("unable to compress: %v", err)
	}
	return bctx, nil
}

// embed embeds the KOTS release into the binary.
func (b *Builder) embed(bctx *buildContext) error {
	if err := b.createKOTSReleaseObjectFile(bctx); err != nil {
		return fmt.Errorf("unable to create release object file: %v", err)
	}
	if err := b.embedKOTSReleaseObjectFile(bctx); err != nil {
		return fmt.Errorf("unable to embed release object file: %v", err)
	}
	return nil
}

// embedKOTSReleaseObjectFile embeds the kots release object file into the binary.
func (b *Builder) embedKOTSReleaseObjectFile(bctx *buildContext) error {
	bundleArg := fmt.Sprintf("sec_bundle=%s", bctx.kotsReleaseObjectPath())
	cmdflag := "--add-section"
	if hasEmbedded, err := bctx.hasEmbeddedRelease(); err != nil {
		return fmt.Errorf("unable to check if binary has embedded release: %v", err)
	} else if hasEmbedded {
		cmdflag = "--update-section"
	}
	command := exec.Command("objcopy", cmdflag, bundleArg, bctx.binaryPath())
	if out, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("unable to embed release object file: %v: %s", err, string(out))
	}
	return nil
}

// createKOTSReleaseObjectFile transform the kots release tar.gz into an object file.
func (b *Builder) createKOTSReleaseObjectFile(bctx *buildContext) error {
	command := exec.Command(
		"objcopy",
		"--input-target", "binary",
		"--output-target", "binary",
		"--rename-section", ".data=sec_bundle",
		bctx.kotsReleasePath(),
		bctx.kotsReleaseObjectPath(),
	)
	if out, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("unable to create release object file: %v: %s", err, string(out))
	}
	return nil
}

// compress compresses the release binary.
func (b *Builder) compress(bctx *buildContext) error {
	binfp, err := os.Open(bctx.binaryPath())
	if err != nil {
		return fmt.Errorf("unable to open binary file: %w", err)
	}
	defer binfp.Close()
	tgz, err := os.Create(bctx.tgzPath())
	if err != nil {
		return fmt.Errorf("unable to create tgz file: %w", err)
	}
	defer tgz.Close()
	gzwriter := gzip.NewWriter(tgz)
	defer gzwriter.Close()
	tarwriter := tar.NewWriter(gzwriter)
	defer tarwriter.Close()
	finfo, err := binfp.Stat()
	if err != nil {
		return fmt.Errorf("unable to stat binary file: %w", err)
	}
	header, err := tar.FileInfoHeader(finfo, "")
	if err != nil {
		return fmt.Errorf("unable to create tar header: %w", err)
	}
	header.Name = filepath.Base(bctx.binaryPath())
	if err := tarwriter.WriteHeader(header); err != nil {
		return fmt.Errorf("unable to write tar header: %w", err)
	}
	if _, err := io.Copy(tarwriter, binfp); err != nil {
		return fmt.Errorf("unable to write tar body: %w", err)
	}
	return nil
}

// NewBuilder returns a new release builder.
func NewBuilder(ctx context.Context) (*Builder, error) {
	src, err := newSource(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to create source: %v", err)
	}
	return &Builder{src}, nil
}
