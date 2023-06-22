package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// New returns a new Supervisor that will start and supervise the provided command with
// the provided arguments.
func New(path string, args []string, opts ...Option) (*Supervisor, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to determine current user: %w", err)
	}
	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current user id: %w", err)
	}
	gid, err := strconv.Atoi(usr.Gid)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current group id: %w", err)
	}
	res := &Supervisor{
		binPath:        path,
		args:           args,
		name:           filepath.Base(path),
		log:            logrus.WithField("component", filepath.Base(path)),
		pidFile:        fmt.Sprintf("/run/replicated/%s.pid", filepath.Base(path)),
		timeoutStop:    5 * time.Second,
		timeoutRespawn: 5 * time.Second,
		uid:            uid,
		gid:            gid,
	}
	for _, opt := range opts {
		opt(res)
	}
	return res, nil
}

// Supervisor is process supervisor, just tries to keep the process running in a while-true loop.
type Supervisor struct {
	name           string
	binPath        string
	log            logrus.FieldLogger
	args           []string
	uid            int
	gid            int
	timeoutStop    time.Duration
	timeoutRespawn time.Duration
	pidFile        string
	cmd            *exec.Cmd
	done           chan bool
	startStopMutex sync.Mutex
	cancel         context.CancelFunc
}

// processWaitQuit waits for a process to exit or a shut down signal returns true if shutdown is requested.
func (s *Supervisor) processWaitQuit(ctx context.Context) (bool, error) {
	waitresult := make(chan error)
	go func() {
		waitresult <- s.cmd.Wait()
	}()
	pidbuf := []byte(strconv.Itoa(s.cmd.Process.Pid))
	if err := os.WriteFile(s.pidFile, pidbuf, 0644); err != nil {
		return false, fmt.Errorf("failed to write file %s: %w", s.pidFile, err)
	}
	defer func() {
		_ = os.Remove(s.pidFile)
	}()

	select {
	case <-ctx.Done():
		if err := s.maybeKillPid(); err != nil {
			return true, fmt.Errorf("failed to kill %s process: %w", s.name, err)
		}
		return true, nil
	case err := <-waitresult:
		if err != nil {
			s.log.Warnf("Failed to waiting for process %q: %v", s.name, err)
			break
		}
		s.log.Warnf("Process %q exited: %s", s.name, s.cmd.ProcessState)
	}
	return false, nil
}

// Supervise Starts supervising the given process.
func (s *Supervisor) Supervise() error {
	s.startStopMutex.Lock()
	defer s.startStopMutex.Unlock()
	if s.cancel != nil {
		s.log.Warn("Supervisor for %q already started", s.name)
		return nil
	}
	dir := filepath.Dir(s.pidFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create run dir: %w", err)
	}
	if err := s.maybeKillPid(); err != nil {
		return err
	}
	var ctx context.Context
	ctx, s.cancel = context.WithCancel(context.Background())
	s.done = make(chan bool)
	if err := s.supervise(ctx); err != nil {
		return fmt.Errorf("failed to supervise %q: %w", s.name, err)
	}
	return nil
}

// DetachAttr creates the proper syscall attributes to run the managed processes.
func (s *Supervisor) detachAttr() *syscall.SysProcAttr {
	var creds *syscall.Credential
	if os.Geteuid() == 0 {
		creds = &syscall.Credential{
			Uid: uint32(s.uid),
			Gid: uint32(s.gid),
		}
	}
	return &syscall.SysProcAttr{
		Setpgid:    true,
		Pgid:       0,
		Credential: creds,
	}
}

// supervise starts the process and waits for it to exit.
func (s *Supervisor) supervise(ctx context.Context) error {
	defer func() {
		close(s.done)
	}()
	s.log.Infof("Starting to supervise %q", s.name)
	s.cmd = exec.Command(s.binPath, s.args...)
	s.cmd.Stdout = logrus.WithField("stream", "stdout").Writer()
	s.cmd.Stdout = logrus.WithField("stream", "stderr").Writer()
	s.cmd.SysProcAttr = s.detachAttr()
	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %q: %w", s.name, err)
	}
	s.log.Infof("Started %q with pid %d", s.name, s.cmd.Process.Pid)

	go func() {
		var restarts int
		for {
			if ended, err := s.processWaitQuit(ctx); err != nil {
				s.log.Errorf("Supervise for %q ended with error: %w", s.name, err)
				return
			} else if ended {
				s.log.Infof("Supervise for %q ended", s.name)
				return
			}
			restarts++
			s.log.Infof("Respawning %q in %s", s.name, s.timeoutRespawn.String())
			select {
			case <-ctx.Done():
				s.log.Infof("Respawn of %q cancelled", s.name)
				return
			case <-time.After(s.timeoutRespawn):
				s.log.Infof("Respawning %q", s.name)
			}
			s.cmd = exec.Command(s.binPath, s.args...)
			s.cmd.Stdout = logrus.WithField("stream", "stdout").Writer()
			s.cmd.Stdout = logrus.WithField("stream", "stderr").Writer()
			s.cmd.SysProcAttr = s.detachAttr()
			if err := s.cmd.Start(); err != nil {
				s.log.Errorf("Failed to respawn %q: %s", s.name, err)
			}
		}
	}()
	return nil
}

// Stop stops the supervised process.
func (s *Supervisor) Stop() error {
	s.startStopMutex.Lock()
	defer s.startStopMutex.Unlock()
	if s.cancel == nil {
		s.log.Warn("Supervised not started")
		return nil
	}
	s.log.Infof("Sending stop message")
	s.cancel()
	s.cancel = nil
	if s.done != nil {
		<-s.done
	}
	s.log.Infof("Supervisor for %q stopped", s.name)
	return nil
}

// killPid signals SIGTERM to a PID and if it's still running after s.timeoutStop sends SIGKILL.
func (s *Supervisor) killPid(pid int) error {
	deadlineTicker := time.NewTicker(s.timeoutStop)
	checkTicker := time.NewTicker(time.Second)
	defer deadlineTicker.Stop()
	defer checkTicker.Stop()
	var stop bool
	for {
		select {
		case <-checkTicker.C:
			s.log.Infof("Sending SIGTERM to pid %d", pid)
			if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
				if err == syscall.ESRCH {
					return nil
				}
				return fmt.Errorf("failed to send sigterm to %d: %w", pid, err)
			}
		case <-deadlineTicker.C:
			stop = true
		}
		if !stop {
			continue
		}
		s.log.Errorf("Process %d still running, sending SIGKILL", pid)
		break
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return fmt.Errorf("failed to send SIGKILL to pid %d: %s", s.cmd.Process.Pid, err)
	}
	return nil
}

// maybeKillPid checks kills the process in the pidFile if it's has the same binary as the supervisor's.
// This function does not delete the old pidFile as this is done by the caller.
func (s *Supervisor) maybeKillPid() error {
	bpid, err := os.ReadFile(s.pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read pid file %s: %v", s.pidFile, err)
	}
	pid, err := strconv.Atoi(string(bpid))
	if err != nil {
		return fmt.Errorf("failed to parse pid file %s: %v", s.pidFile, err)
	}
	if should, err := s.shouldKillProcess(pid); err != nil {
		return fmt.Errorf("failed to assess if we should kill pid %d: %w", pid, err)
	} else if !should {
		return fmt.Errorf("pid %d is not a %q process", pid, s.name)
	}
	return s.killPid(pid)
}

// shouldKillProcess returns true if the proccess with the provided pid should be killed. By should be
// killed is understood as the command for process with the given pid matches the command we are
// supervising.
func (s *Supervisor) shouldKillProcess(pid int) (bool, error) {
	cmdline, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to read process %d cmdline: %v", pid, err)
	}
	if cmd := strings.Split(string(cmdline), "\x00"); len(cmd) > 0 {
		return cmd[0] == s.binPath, nil
	}
	return false, nil
}
