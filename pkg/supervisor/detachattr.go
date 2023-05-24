//go:build !windows
// +build !windows

package supervisor

import (
	"os"
	"syscall"
)

// DetachAttr creates the proper syscall attributes to run the managed processes
func DetachAttr(uid, gid int) *syscall.SysProcAttr {
	var creds *syscall.Credential

	if os.Geteuid() == 0 {
		creds = &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		}
	}

	return &syscall.SysProcAttr{
		Setpgid:    true,
		Pgid:       0,
		Credential: creds,
	}
}
