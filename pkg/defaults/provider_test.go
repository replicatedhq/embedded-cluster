package defaults

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "helmvm")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpdir) }()
	def := NewProvider(tmpdir)
	assert.DirExists(t, def.K0sctlBinsSubDir(), "k0s binary dir should exist")
	assert.DirExists(t, def.ConfigSubDir(), "config dir should exist")
	assert.DirExists(t, def.HelmVMBinsSubDir(), "helmvm binary dir should exist")
}

func TestDecentralizedInstall(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "helmvm")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpdir) }()
	def := NewProvider(tmpdir)
	assert.False(t, def.DecentralizedInstall(), "default should be centralized")
	err = def.SetInstallAsDecentralized()
	assert.NoError(t, err)
	assert.True(t, def.DecentralizedInstall(), "unable to set decentralized install")
}

func TestFileNameForImage(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "helmvm")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpdir) }()
	def := NewProvider(tmpdir)
	for img, exp := range map[string]string{
		"nginx:latest":                   "nginx-latest.tar",
		"nginx":                          "nginx.tar",
		"nginx@sha256:1234567890":        "nginx-sha256-1234567890.tar",
		"docker.io/library/nginx:latest": "docker.io-library-nginx-latest.tar",
		"quay.io/project/image:v123":     "quay.io-project-image-v123.tar",
	} {
		result := def.FileNameForImage(img)
		assert.Equal(t, exp, result, "unexpected filename for image %s", img)
	}

}

func TestPreferredNodeIPAddress(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "helmvm")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpdir) }()
	def := NewProvider(tmpdir)
	ip, err := def.PreferredNodeIPAddress()
	assert.NoError(t, err)
	assert.NotEmpty(t, ip, "ip address should not be empty")
}

func TestEnsureAllDirectoriesAreInsideBase(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "helmvm")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpdir) }()
	def := NewProvider(tmpdir)
	for _, fn := range []func() string{
		def.K0sctlBinsSubDir,
		def.HelmVMBinsSubDir,
		def.HelmVMLogsSubDir,
		def.K0sctlApplyLogPath,
		def.SSHKeyPath,
		def.SSHAuthorizedKeysPath,
		def.K0sBinaryPath,
		def.ConfigSubDir,
		def.SSHConfigSubDir,
	} {
		assert.Contains(t, fn(), tmpdir, "directory should be inside base")
		count := strings.Count(fn(), tmpdir)
		assert.Equal(t, 1, count, "base directory should not repeat")
	}
}
