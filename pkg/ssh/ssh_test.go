package ssh

import (
	"os"
	"path"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/stretchr/testify/assert"
)

func TestAllowLocalSSH(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "helmvm")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	def := defaults.NewProvider(tmpdir)
	ssh := SSH{def}
	err = ssh.AllowLocalSSH()
	assert.NoError(t, err)
	private := path.Join(def.SSHConfigSubDir(), "helmvm_rsa")
	assert.FileExists(t, private, "private key should exist")
	public := path.Join(def.SSHConfigSubDir(), "helmvm_rsa.pub")
	assert.FileExists(t, public, "public key should exist")
	authkeys := path.Join(def.SSHConfigSubDir(), "authorized_keys")
	assert.FileExists(t, authkeys, "authorized keys file should exist")
}

func TestKeysCreatedOnlyOnce(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "helmvm")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	def := defaults.NewProvider(tmpdir)
	ssh := SSH{def}
	err = ssh.AllowLocalSSH()
	assert.NoError(t, err)
	private := path.Join(def.SSHConfigSubDir(), "helmvm_rsa")
	oldfinfo, err := os.Stat(private)
	assert.NoError(t, err)
	err = ssh.AllowLocalSSH()
	assert.NoError(t, err)
	newfinfo, err := os.Stat(private)
	assert.NoError(t, err)
	assert.Equal(t, oldfinfo.ModTime(), newfinfo.ModTime(), "private key should not be updated")
}

func TestAppendToAuthorized(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "helmvm")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	def := defaults.NewProvider(tmpdir)
	ssh := SSH{def}
	authkeypath := path.Join(def.SSHConfigSubDir(), "authorized_keys")
	err = os.WriteFile(authkeypath, []byte("foo\n"), 0600)
	assert.NoError(t, err)
	err = ssh.AllowLocalSSH()
	assert.NoError(t, err)
	content, err := os.ReadFile(authkeypath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "foo", "authorized keys should contain original content")
}
