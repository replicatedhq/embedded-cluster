package embed

import (
	"debug/elf"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewElfBuilder(t *testing.T) {
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", "")
	_, err := NewElfBuilder("")
	assert.Error(t, err, "expected an error when 'objcopy' is not in the PATH")
	os.Setenv("PATH", originalPath)
	tmpDir, err := os.MkdirTemp("", "test")
	assert.NoError(t, err, "expected no error when creating a temp directory")
	defer os.RemoveAll(tmpDir)
	builder, err := NewElfBuilder(tmpDir)
	assert.NoError(t, err, "expected no error when 'objcopy' is in the PATH and temp directory can be created")
	assert.True(t, strings.HasPrefix(builder.baseDir, tmpDir), "The base directory should be within the specified tmpDir")
	err = builder.Close()
	assert.NoError(t, err, "expected no error when closing the builder")
	_, err = os.Stat(builder.baseDir)
	assert.True(t, os.IsNotExist(err), "expected the base directory to be removed when closing the builder")
}

func Test_withBinary(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "elfbuilder_test")
	assert.NoError(t, err, "expected no error when creating a temp directory")
	defer os.RemoveAll(tmpDir)
	builder := &ElfBuilder{baseDir: tmpDir}
	binpath, err := exec.LookPath("ls")
	assert.NoError(t, err, "expected no error when looking for 'ls' in the PATH")
	err = builder.withBinary(binpath)
	assert.NoError(t, err, "expected no error in successful copy")
	copiedFilePath := path.Join(tmpDir, "embedded-cluster")
	_, err = os.Stat(copiedFilePath)
	assert.NoError(t, err, "expected the file to exist in the destination path")
	err = builder.withBinary("nonexistentfile")
	assert.Error(t, err, "expected an error when the source file does not exist")
}

func Test_withRelease(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "elfbuilder_withrelease_test")
	assert.NoError(t, err, "expected no error when creating a temp directory")
	defer os.RemoveAll(tmpDir)
	builder := &ElfBuilder{baseDir: tmpDir}
	err = builder.withRelease("nonexistentfile")
	assert.Error(t, err, "expected an error when the release file does not exist")
	err = builder.withRelease("testdata/release.tar.gz")
	assert.NoError(t, err, "expected no error with a valid release file")
	outputFile := path.Join(tmpDir, "release.o")
	_, err = os.Stat(outputFile)
	assert.NoError(t, err, "expected the output file to be created")
}

func TestElfBuilderBuild(t *testing.T) {
	binpath, err := exec.LookPath("ls")
	assert.NoError(t, err, "expected no error when looking for 'ls' in the PATH")
	builder, err := NewElfBuilder("")
	assert.NoError(t, err, "expected no error when creating a new builder")
	newbin, err := builder.Build(binpath, "testdata/release.tar.gz")
	assert.NoError(t, err, "expected no error when building a valid binary")
	dstfile := path.Join(builder.baseDir, "embedded-cluster")
	fp, err := elf.Open(dstfile)
	assert.NoError(t, err, "expected no error when opening the built binary")
	defer fp.Close()
	section := fp.Section("sec_bundle")
	assert.NotNil(t, section, "expected the bundle section to be present")
	data, err := section.Data()
	assert.NoError(t, err, "expected no error when reading the bundle section")
	releaseData, err := os.ReadFile("testdata/release.tar.gz")
	assert.NoError(t, err, "expected no error when reading the release file")
	assert.Equal(t, releaseData, data, "expected the bundle section to contain the release file data")
	_, err = io.Copy(io.Discard, newbin)
	assert.NoError(t, err, "expected no error when copying the built binary")
	assert.NoError(t, builder.Close(), "expected no error when closing the builder")
}

func TestExtractFromBinary(t *testing.T) {
	binpath, err := exec.LookPath("ls")
	assert.NoError(t, err, "expected no error when looking for 'ls' in the PATH")
	builder, err := NewElfBuilder("")
	assert.NoError(t, err, "expected no error when creating a new builder")
	newbin, err := builder.Build(binpath, "testdata/release.tar.gz")
	assert.NoError(t, err, "expected no error when building a valid binary")
	dstfile, err := os.CreateTemp("", "elfbuilder_test")
	assert.NoError(t, err, "expected no error when creating a temp file")
	defer os.Remove(dstfile.Name())
	defer dstfile.Close()
	_, err = io.Copy(dstfile, newbin)
	assert.NoError(t, err, "expected no error when copying the built binary")
	target, err := ExtractFromBinary(dstfile.Name())
	assert.NoError(t, err, "expected no error when extracting the bundle from the binary")
	source, err := os.ReadFile("testdata/release.tar.gz")
	assert.NoError(t, err, "expected no error when reading the release file")
	assert.Equal(t, source, target, "expected the extracted bundle to be the same as the source")
}
