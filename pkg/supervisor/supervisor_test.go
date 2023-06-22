package supervisor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	usr, err := user.Current()
	assert.NoError(t, err)
	uid, err := strconv.Atoi(usr.Uid)
	assert.NoError(t, err)
	gid, err := strconv.Atoi(usr.Gid)
	assert.NoError(t, err)
	s, err := New("/bin/cat", []string{"-"})
	assert.NoError(t, err)
	assert.Equal(t, "/bin/cat", s.binPath)
	assert.Equal(t, []string{"-"}, s.args)
	assert.Equal(t, uid, s.uid)
	assert.Equal(t, gid, s.gid)
	assert.NotZero(t, s.timeoutRespawn)
	assert.NotZero(t, s.timeoutStop)
	assert.NotNil(t, s.log)
	assert.Equal(t, s.name, "cat")
}

func TestSupervise(t *testing.T) {
	s, err := New("/usr/bin/sleep", []string{"60"}, WithPIDFile("/tmp/cat.pid"))
	assert.NoError(t, err)
	fmt.Println(time.Now())
	assert.NoError(t, s.Supervise())
	fmt.Println(time.Now())
	time.Sleep(time.Second)
	assert.NoError(t, s.Stop())
}

func Test_shouldKillProcess(t *testing.T) {
	s, err := New("", nil)
	assert.NoError(t, err)
	should, err := s.shouldKillProcess(-999)
	assert.NoError(t, err)
	assert.False(t, should)
	should, err = s.shouldKillProcess(os.Getpid())
	assert.NoError(t, err)
	assert.False(t, should)
	s, err = New("/usr/bin/sleep", []string{"60"}, WithPIDFile("/tmp/cat.pid"))
	assert.NoError(t, err)
	assert.NoError(t, s.Supervise())
	time.Sleep(time.Second)
	should, err = s.shouldKillProcess(s.cmd.Process.Pid)
	assert.NoError(t, err)
	assert.True(t, should)
	assert.NoError(t, s.Stop())
}

func Test_killPid(t *testing.T) {
	s, err := New("/usr/bin/sleep", []string{"60"}, WithPIDFile("/tmp/cat.pid"))
	assert.NoError(t, err)
	assert.NoError(t, s.Supervise())
	time.Sleep(time.Second)
	assert.NoError(t, s.killPid(s.cmd.Process.Pid))
	assert.NoError(t, s.Stop())
	cmd := exec.Command("/usr/bin/sleep", "60")
	assert.NoError(t, cmd.Start())
	assert.NoError(t, s.killPid(cmd.Process.Pid))
}

func Test_processWaitQuit(t *testing.T) {
	cmd := exec.Command("/usr/bin/sleep", "5")
	assert.NoError(t, cmd.Start())
	ppath := "/does-not-exist/does-not-exist.pid"
	s := Supervisor{pidFile: ppath, cmd: cmd}
	s.pidFile = "/does-not-exist/does-not-exist.pid"
	_, err := s.processWaitQuit(context.Background())
	assert.Error(t, err, "failed to write file %[1]s: open %[1]s: no such file or directory", ppath)
}

var goodScript = `#!/bin/sh
date >> /tmp/good_supervisor_test.log
`

func TestExitingProcess(t *testing.T) {
	assert.NoError(t, os.RemoveAll("/tmp/good_supervisor_test.log"))
	assert.NoError(t, os.WriteFile("/tmp/good_supervisor_test.sh", []byte(goodScript), 0755))
	script := "/tmp/good_supervisor_test.sh"
	pidpath := "/tmp/good_supervisor_test.pid"
	s, err := New(script, nil, WithPIDFile(pidpath), WithTimeoutRespawn(100*time.Millisecond))
	assert.NoError(t, err)
	assert.NoError(t, s.Supervise())
	time.Sleep(time.Second)
	assert.NoError(t, s.Stop())
	assert.FileExists(t, "/tmp/good_supervisor_test.log")
	data, err := os.ReadFile("/tmp/good_supervisor_test.log")
	assert.NoError(t, err)
	lines := bytes.Split(data, []byte("\n"))
	assert.Greater(t, len(lines), 1)
}

var badScript = `#!/bin/sh
date >> /tmp/bad_supervisor_test.log
exit 3
`

func TestCrashingProcess(t *testing.T) {
	assert.NoError(t, os.RemoveAll("/tmp/bad_supervisor_test.log"))
	assert.NoError(t, os.WriteFile("/tmp/bad_supervisor_test.sh", []byte(badScript), 0755))
	script := "/tmp/good_supervisor_test.sh"
	pidpath := "/tmp/good_supervisor_test.pid"
	s, err := New(script, nil, WithPIDFile(pidpath), WithTimeoutRespawn(100*time.Millisecond))
	assert.NoError(t, err)
	assert.NoError(t, s.Supervise())
	time.Sleep(time.Second)
	assert.NoError(t, s.Stop())
	assert.FileExists(t, "/tmp/bad_supervisor_test.log")
	data, err := os.ReadFile("/tmp/bad_supervisor_test.log")
	assert.NoError(t, err)
	lines := bytes.Split(data, []byte("\n"))
	assert.Greater(t, len(lines), 1)
}

func Test_maybeKillPid(t *testing.T) {
	ppath := "/tmp/maybe_kill_pid.pid"
	assert.NoError(t, os.WriteFile(ppath, []byte("abc"), 0644))
	s, err := New("/usr/bin/sleep", []string{"60"}, WithPIDFile(ppath))
	assert.NoError(t, err)
	assert.Error(
		t,
		s.maybeKillPid(),
		"failed to parse pid file %[1]s: strconv.Atoi: parsing \"abc\": invalid syntax",
		ppath,
	)
	assert.NoError(t, os.WriteFile(ppath, []byte("1"), 0644))
	s, err = New("/usr/bin/sleep", []string{"60"}, WithPIDFile(ppath))
	assert.NoError(t, err)
	assert.Error(t, s.maybeKillPid(), `pid 1 is not a "sleep" process`)
}
