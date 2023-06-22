package supervisor

import (
	"fmt"
	"time"
)

// Option sets an option on a Supervisor reference.
type Option func(*Supervisor)

// WithName sets the name of the Supervisor.
func WithName(name string) Option {
	return func(s *Supervisor) {
		s.name = name
		s.log = s.log.WithField("component", name)
		s.pidFile = fmt.Sprintf("/run/replicated/%s.pid", name)
	}
}

// WithTimeoutRespawn sets the timeout for respawning a process.
func WithTimeoutRespawn(timeoutRespawn time.Duration) Option {
	return func(s *Supervisor) {
		s.timeoutRespawn = timeoutRespawn
	}
}

// WithTimeoutStop sets the timeout for stopping a process.
func WithTimeoutStop(timeoutStop time.Duration) Option {
	return func(s *Supervisor) {
		s.timeoutStop = timeoutStop
	}
}

// WithUID sets the UID of the Supervisor.
func WithUID(uid int) Option {
	return func(s *Supervisor) {
		s.uid = uid
	}
}

// WithGID sets the GID of the Supervisor.
func WithGID(gid int) Option {
	return func(s *Supervisor) {
		s.gid = gid
	}
}

// WithPIDFile sets the PID file of the Supervisor.
func WithPIDFile(pidFile string) Option {
	return func(s *Supervisor) {
		s.pidFile = pidFile
	}
}
