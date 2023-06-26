package supervisor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithName(t *testing.T) {
	s, err := New("/bin/cat", []string{"-"}, WithName("CAT"))
	assert.NoError(t, err)
	assert.Equal(t, "CAT", s.name)
	assert.Equal(t, "/run/replicated/CAT.pid", s.pidFile)
}

func TestWithTimeoutRespawn(t *testing.T) {
	s, err := New("/bin/cat", []string{"-"}, WithTimeoutRespawn(time.Second))
	assert.NoError(t, err)
	assert.Equal(t, time.Second, s.timeoutRespawn)
}

func TestWithTimeoutStop(t *testing.T) {
	s, err := New("/bin/cat", []string{"-"}, WithTimeoutStop(time.Second))
	assert.NoError(t, err)
	assert.Equal(t, time.Second, s.timeoutStop)
}

func TestWithGID(t *testing.T) {
	s, err := New("/bin/cat", []string{"-"}, WithGID(1000))
	assert.NoError(t, err)
	assert.Equal(t, 1000, s.gid)
}

func TestWithUID(t *testing.T) {
	s, err := New("/bin/cat", []string{"-"}, WithUID(1000))
	assert.NoError(t, err)
	assert.Equal(t, 1000, s.uid)
}

func TestWithPIDFile(t *testing.T) {
	s, err := New("/bin/cat", []string{"-"}, WithPIDFile("/run/abc.pid"))
	assert.NoError(t, err)
	assert.Equal(t, "/run/abc.pid", s.pidFile)
}
