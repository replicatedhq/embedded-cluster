package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeHelpers records every command and can block one of them until the
// caller's context is cancelled, reproducing a `k0s reset` stuck on a container
// runtime call that never returns.
type fakeHelpers struct {
	mu      sync.Mutex
	calls   []string
	blockOn string

	// runCommandFn, when set, overrides the default RunCommand behavior so a
	// test can simulate specific binaries/args failing or returning output.
	runCommandFn func(bin string, args ...string) (string, error)
}

func (f *fakeHelpers) record(bin string, args ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.TrimSpace(bin+" "+strings.Join(args, " ")))
}

func (f *fakeHelpers) ran(prefix string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

// ranCount returns how many recorded calls contain substr. Calls to the fake
// k0s binary are recorded with its full temp-file path, so unlike ran (which
// matches binaries invoked by their literal name, e.g. "pkill"), this must
// match on a substring rather than a prefix.
func (f *fakeHelpers) ranCount(substr string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, c := range f.calls {
		if strings.Contains(c, substr) {
			count++
		}
	}
	return count
}

func (f *fakeHelpers) RunCommand(bin string, args ...string) (string, error) {
	f.record(bin, args...)
	if f.runCommandFn != nil {
		return f.runCommandFn(bin, args...)
	}
	if bin == "systemctl" {
		// Make stopAndResetK0s believe the controller unit exists.
		return "k0scontroller.service enabled", nil
	}
	return "", nil
}

func (f *fakeHelpers) RunCommandWithOptions(opts helpers.RunCommandOptions, bin string, args ...string) error {
	f.record(bin, args...)
	if f.blockOn != "" && strings.Contains(strings.Join(args, " "), f.blockOn) {
		ctx := opts.Context
		if ctx == nil {
			ctx = context.Background()
		}
		<-ctx.Done()
		return ctx.Err()
	}
	if f.runCommandFn != nil {
		out, err := f.runCommandFn(bin, args...)
		if opts.Stdout != nil && out != "" {
			_, _ = opts.Stdout.Write([]byte(out))
		}
		return err
	}
	return nil
}

func (f *fakeHelpers) IsSystemdServiceActive(context.Context, string) (bool, error) {
	return false, nil
}

// installFakeHelpers swaps in the fake for the duration of the test and points
// k0sBinPath at a file that exists, so stopAndResetK0s does not bail out early.
func installFakeHelpers(t *testing.T, f *fakeHelpers) {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "k0s")
	require.NoError(t, os.WriteFile(bin, []byte("fake k0s"), 0755))

	origBin, origHelpers := k0sBinPath, helpers.HelpersInterface(&helpers.Helpers{})
	k0sBinPath = bin
	helpers.Set(f)
	t.Cleanup(func() {
		k0sBinPath = origBin
		helpers.Set(origHelpers)
	})
}

// TestStopAndResetK0s_TimesOut covers the failure Rockwell hit: `k0s reset`
// blocks forever on a containerd that stopped responding, so reset never
// returns and every cleanup step after it is skipped. It must give up instead.
func TestStopAndResetK0s_TimesOut(t *testing.T) {
	origTimeout, origLead := k0sResetTimeout, k0sResetDumpLead
	k0sResetTimeout, k0sResetDumpLead = 400*time.Millisecond, 200*time.Millisecond
	t.Cleanup(func() { k0sResetTimeout, k0sResetDumpLead = origTimeout, origLead })

	f := &fakeHelpers{blockOn: "reset --data-dir"}
	installFakeHelpers(t, f)

	done := make(chan error, 1)
	go func() { done <- stopAndResetK0s("/var/lib/embedded-cluster/k0s") }()

	var err error
	select {
	case err = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("stopAndResetK0s did not return: the k0s reset call is still unbounded")
	}

	require.Error(t, err, "should report that k0s reset timed out")
	assert.Contains(t, err.Error(), "timed out")

	// k0s is asked for a goroutine dump before the deadline kills it; its output
	// is streamed to the log, so this is what identifies the stuck CRI call.
	assert.True(t, f.ran("pkill -QUIT -f k0s reset --data-dir"),
		"should ask k0s for a goroutine dump before the deadline")
}

// TestForceK0sTeardown asserts reset performs the cleanup k0s did not: killing
// the processes holding the mounts, then detaching them so the directory
// removals that follow do not fail with EBUSY.
func TestForceK0sTeardown(t *testing.T) {
	f := &fakeHelpers{}
	installFakeHelpers(t, f)

	homeDir := "/var/lib/embedded-cluster"
	k0sDataDir := "/var/lib/embedded-cluster/k0s"
	forceK0sTeardown(homeDir, k0sDataDir)

	for _, proc := range []string{"k0s", "kube-apiserver", "kubelet", "containerd"} {
		assert.True(t, f.ran("pkill -9 -f "+proc), "should force-kill orphaned %s", proc)
	}

	for _, dir := range []string{homeDir, k0sDataDir, k0sRunDir} {
		assert.True(t, f.ran(strings.TrimSpace("sh -c "+unmountScript+" sh "+dir)),
			"should unmount everything below %s", dir)
	}

	// Calico state is torn down by k0s's cni cleanup step, which did not run.
	assert.True(t, f.ran("ip link delete vxlan.calico"),
		"should remove the calico vxlan interface")
	assert.True(t, f.ran("sh -c ip link show"),
		"should remove the calico veth interfaces")
	assert.True(t, f.ran("sh -c ip route show table all"),
		"should remove the calico blackhole routes")
}

// speedUpEtcdLeaveRetries removes the retry sleeps from leaveEtcdCluster for
// the duration of the test.
func speedUpEtcdLeaveRetries(t *testing.T) {
	t.Helper()
	orig := etcdLeaveRetryDelay
	etcdLeaveRetryDelay = time.Millisecond
	t.Cleanup(func() { etcdLeaveRetryDelay = orig })
}

// setLogrusOutput sends the global logrus logger's output to w at Info level
// for the rest of the test, restoring the previous output and level after.
// No test in this package runs in parallel, so swapping the global is safe.
func setLogrusOutput(t *testing.T, w io.Writer) {
	t.Helper()
	origOut, origLevel := logrus.StandardLogger().Out, logrus.GetLevel()
	logrus.SetOutput(w)
	logrus.SetLevel(logrus.InfoLevel)
	t.Cleanup(func() {
		logrus.SetOutput(origOut)
		logrus.SetLevel(origLevel)
	})
}

func TestLeaveEtcdCluster(t *testing.T) {
	setLogrusOutput(t, io.Discard)
	useK0sControllerFiles(t, true, "")

	// Sole etcd member: nothing to leave.
	t.Run("SoleMember", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return `{"members":{"node-a":"https://node-a:2380"}}`, nil
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}
		removed, warning := h.leaveEtcdCluster(context.Background())

		assert.True(t, removed)
		assert.Empty(t, warning)
		assert.Equal(t, 0, f.ranCount("etcd leave"), "should not attempt to leave when already the sole member")
	})

	// Happy path: other members exist and the leave succeeds.
	t.Run("Success", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return `{"members":{"node-a":"https://node-a:2380","node-b":"https://node-b:2380"}}`, nil
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}
		removed, warning := h.leaveEtcdCluster(context.Background())

		assert.True(t, removed)
		assert.Empty(t, warning)
		assert.Equal(t, 1, f.ranCount("etcd leave"), "should actually attempt to leave the cluster")
	})

	// Hostname missing from member-list: reset still makes one defensive leave,
	// and etcd's "member not found" from it confirms removal.
	t.Run("AlreadyRemoved", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return `{"members":{"node-b":"https://node-b:2380","node-c":"https://node-c:2380"}}`, nil
				}
				if len(args) == 2 && args[0] == "etcd" && args[1] == "leave" {
					return "", errEtcdMemberNotFound
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}
		removed, warning := h.leaveEtcdCluster(context.Background())

		assert.True(t, removed)
		assert.Empty(t, warning)
		assert.Equal(t, 1, f.ranCount("etcd leave"), "should make a defensive leave attempt rather than trusting the member-list key match alone")
	})

	// Defensive leave gets k0s's "peer not found": with no member-list URL to check
	// it against, the node may still be a voter, so reset warns.
	t.Run("AlreadyRemovedPeerNotFoundIsInconclusive", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return `{"members":{"node-a.example":"https://10.0.0.1:2380","node-b":"https://10.0.0.2:2380"}}`, nil
				}
				if len(args) == 2 && args[0] == "etcd" && args[1] == "leave" {
					return "", errK0sPeerNotFound("https://10.0.0.9:2380")
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}
		removed, warning := h.leaveEtcdCluster(context.Background())

		require.False(t, removed)
		assert.Contains(t, warning, "peer not found: https://10.0.0.9:2380")
		assert.Contains(t, warning, `run 'k0s etcd member-list' to find "node-a"'s peer address`)
		assert.Equal(t, 1, f.ranCount("etcd leave"), "the defensive leave is a single attempt")
	})

	// Hostname missing from member-list but the node is still a member: the
	// defensive leave removes it, with no retry.
	t.Run("AlreadyRemovedButActuallyStillAMember", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return `{"members":{"node-b":"https://node-b:2380","node-c":"https://node-c:2380"}}`, nil
				}
				if len(args) == 2 && args[0] == "etcd" && args[1] == "leave" {
					return "", nil
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}
		removed, warning := h.leaveEtcdCluster(context.Background())

		assert.True(t, removed)
		assert.Empty(t, warning)
		assert.Equal(t, 1, f.ranCount("etcd leave"), "the defensive leave attempt should not be retried")
	})

	// A defensive leave that fails for another reason (e.g. unreachable etcd)
	// must warn, not count as removed.
	t.Run("AlreadyRemovedButLeaveFailsForUnrelatedReason", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return `{"members":{"node-b":"https://node-b:2380","node-c":"https://node-c:2380"}}`, nil
				}
				if len(args) == 2 && args[0] == "etcd" && args[1] == "leave" {
					return "", errors.New("context deadline exceeded")
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}
		removed, warning := h.leaveEtcdCluster(context.Background())

		require.False(t, removed)
		assert.Contains(t, warning, "node-a")
		assert.Contains(t, warning, "member-list",
			"no peer address was ever resolved, so the warning must point at how to find the real one")
		assert.Equal(t, 1, f.ranCount("etcd leave"), "the defensive leave attempt should not be retried")
	})

	// member-list is keyed by hostname: k0s sets etcd's --name to os.Hostname()
	// and keys the member-list JSON by member name (k0s pkg/component/controller/etcd.go,
	// cmd/etcd/list.go at the version pinned in go.mod).
	t.Run("MemberListKeyIsHostname", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		const hostname = "ip-10-0-12-34.us-west-2.compute.internal"
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return `{"members":{"` + hostname + `":"https://` + hostname + `:2380","node-b":"https://node-b:2380"}}`, nil
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: hostname}
		removed, warning := h.leaveEtcdCluster(context.Background())

		assert.True(t, removed)
		assert.Empty(t, warning)
		assert.Equal(t, 1, f.ranCount("etcd leave"),
			"a member-list keyed by this node's real hostname must be recognized as present, taking the confirmed leave path rather than the defensive one")
	})

	// The only etcd command reset may change membership with is a bare
	// `k0s etcd leave`, which removes this node's own member. Every k0s call in
	// every outcome must be that or `etcd member-list`: no --peer-address, no
	// other etcd command, and at most 3 leave attempts.
	t.Run("OnlyRunsBareEtcdCommands", func(t *testing.T) {
		const (
			listed   = `{"members":{"node-a":"https://node-a:2380","node-b":"https://node-b:2380"}}`
			unlisted = `{"members":{"node-b":"https://node-b:2380","node-c":"https://node-c:2380"}}`
			sole     = `{"members":{"node-a":"https://node-a:2380"}}`
		)
		scenarios := []struct {
			name       string
			memberList string
			listErr    error
			leaveErr   error
		}{
			{name: "leave succeeds", memberList: listed},
			{name: "sole member", memberList: sole},
			{name: "leave keeps failing", memberList: listed, leaveErr: errors.New("connection refused")},
			{name: "local etcd stopped", memberList: listed, leaveErr: errors.New("etcdserver: server stopped")},
			{name: "peer not found for another URL", memberList: listed, leaveErr: errK0sPeerNotFound("https://10.0.0.9:2380")},
			{name: "member removed mid-call", memberList: listed, leaveErr: errEtcdMemberNotFound},
			{name: "defensive leave, member not found", memberList: unlisted, leaveErr: errEtcdMemberNotFound},
			{name: "defensive leave, peer not found", memberList: unlisted, leaveErr: errK0sPeerNotFound("https://node-a:2380")},
			{name: "defensive leave, other error", memberList: unlisted, leaveErr: errors.New("connection refused")},
			{name: "member-list fails", listErr: errors.New("context deadline exceeded")},
		}

		for _, sc := range scenarios {
			t.Run(sc.name, func(t *testing.T) {
				speedUpEtcdLeaveRetries(t)
				f := &fakeHelpers{
					runCommandFn: func(bin string, args ...string) (string, error) {
						if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
							return sc.memberList, sc.listErr
						}
						if len(args) >= 2 && args[0] == "etcd" && args[1] == "leave" {
							return "", sc.leaveErr
						}
						return "", nil
					},
				}
				installFakeHelpers(t, f)

				h := &hostInfo{Hostname: "node-a"}
				h.leaveEtcdCluster(context.Background())

				leaves := 0
				for _, c := range f.calls {
					switch strings.TrimPrefix(c, k0sBinPath+" ") {
					case "etcd member-list":
					case "etcd leave":
						leaves++
					default:
						t.Errorf("unexpected k0s call %q: reset may only list members or run a bare leave", c)
					}
				}
				assert.LessOrEqual(t, leaves, 3, "at most 3 leave attempts")
			})
		}
	})

	// member-list fails: reset can't tell, so it warns (sc-139620).
	t.Run("MemberListFails", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return "", errors.New("context deadline exceeded")
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}
		removed, warning := h.leaveEtcdCluster(context.Background())

		require.False(t, removed)
		assert.Contains(t, warning, "node-a")
		assert.Contains(t, warning, "member-list",
			"no peer address was ever resolved, so the warning must point at how to find the real one")
		assert.Equal(t, 0, f.ranCount("etcd leave"), "should not attempt to leave without a confirmed member list")
	})

	// member-list fails: the warning falls back to the configured peer address.
	t.Run("MemberListFailsUsesConfiguredPeerAddress", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		useK0sControllerFiles(t, true, "spec:\n  storage:\n    etcd:\n      peerAddress: 10.0.0.12\n")
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return "", errors.New("context deadline exceeded")
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}
		removed, warning := h.leaveEtcdCluster(context.Background())

		require.False(t, removed)
		assert.Contains(t, warning, "k0s etcd leave --peer-address 10.0.0.12")
	})

	// A stopped local etcd isn't proof the member left, so reset warns.
	t.Run("EtcdStopped", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return `{"members":{"node-a":"https://node-a:2380","node-b":"https://node-b:2380"}}`, nil
				}
				if len(args) == 2 && args[0] == "etcd" && args[1] == "leave" {
					return "", errors.New("etcdserver: server stopped")
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}
		removed, warning := h.leaveEtcdCluster(context.Background())

		require.False(t, removed)
		assert.Contains(t, warning, "node-a")
		assert.Contains(t, warning, "k0s etcd leave --peer-address node-a")
	})

	// The remediation command uses the member's etcd peer address from
	// member-list, not the hostname, since the two can differ.
	t.Run("RemediationUsesEtcdPeerAddressNotHostname", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		const hostname = "ip-10-0-12-34.us-west-2.compute.internal"
		const peerAddress = "10.0.12.34"
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return `{"members":{"` + hostname + `":"https://` + peerAddress + `:2380","node-b":"https://node-b:2380"}}`, nil
				}
				if len(args) == 2 && args[0] == "etcd" && args[1] == "leave" {
					return "", errors.New("context deadline exceeded")
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: hostname}
		removed, warning := h.leaveEtcdCluster(context.Background())

		require.False(t, removed)
		assert.Contains(t, warning, hostname, "the warning should still identify the node by its hostname")
		assert.Contains(t, warning, "k0s etcd leave --peer-address "+peerAddress,
			"the remediation command must use the etcd peer address from the member list, not the node's hostname")
		assert.NotContains(t, warning, "--peer-address "+hostname,
			"the remediation command must not fall back to the hostname when the real peer address is known")
	})

	// A member-list call that hangs (e.g. lost quorum) is cut off by the timeout.
	t.Run("MemberListBlocks", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		speedUpEtcdCmdTimeout(t)

		f := &fakeHelpers{blockOn: "member-list"}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}

		done := make(chan struct{})
		var removed bool
		var warning string
		go func() {
			removed, warning = h.leaveEtcdCluster(context.Background())
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("leaveEtcdCluster did not return: the k0s etcd member-list call is still unbounded")
		}

		require.False(t, removed)
		assert.Contains(t, warning, "node-a")
		assert.Equal(t, 0, f.ranCount("etcd leave"), "should not attempt to leave without a confirmed member list")
	})

	// A leave call that hangs is cut off by the timeout.
	t.Run("LeaveBlocks", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		speedUpEtcdCmdTimeout(t)

		f := &fakeHelpers{
			blockOn: "etcd leave",
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return `{"members":{"node-a":"https://node-a:2380","node-b":"https://node-b:2380"}}`, nil
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}

		done := make(chan struct{})
		var removed bool
		var warning string
		go func() {
			removed, warning = h.leaveEtcdCluster(context.Background())
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("leaveEtcdCluster did not return: the k0s etcd leave call is still unbounded")
		}

		require.False(t, removed)
		assert.Contains(t, warning, "node-a")
	})

	// A leave that keeps failing warns after the retries run out.
	t.Run("LeaveFailsAfterRetries", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return `{"members":{"node-a":"https://node-a:2380","node-b":"https://node-b:2380"}}`, nil
				}
				if len(args) == 2 && args[0] == "etcd" && args[1] == "leave" {
					return "", errors.New("connection refused")
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}
		removed, warning := h.leaveEtcdCluster(context.Background())

		require.False(t, removed)
		assert.Contains(t, warning, "node-a")
		assert.Contains(t, warning, "k0s etcd leave --peer-address node-a")
	})

	// The direct leave races a removal that already happened: that confirms removal.
	t.Run("DirectLeaveAlreadyGone", func(t *testing.T) {
		for name, leaveErr := range map[string]error{
			"k0s peer not found":    errK0sPeerNotFound("https://node-a:2380"),
			"etcd member not found": errEtcdMemberNotFound,
		} {
			t.Run(name, func(t *testing.T) {
				speedUpEtcdLeaveRetries(t)
				f := &fakeHelpers{
					runCommandFn: func(bin string, args ...string) (string, error) {
						if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
							return `{"members":{"node-a":"https://node-a:2380","node-b":"https://node-b:2380"}}`, nil
						}
						if len(args) == 2 && args[0] == "etcd" && args[1] == "leave" {
							return "", leaveErr
						}
						return "", nil
					},
				}
				installFakeHelpers(t, f)

				h := &hostInfo{Hostname: "node-a"}
				removed, warning := h.leaveEtcdCluster(context.Background())

				assert.True(t, removed)
				assert.Empty(t, warning)
				assert.Equal(t, 1, f.ranCount("etcd leave"), "should not retry once removal is confirmed")
			})
		}
	})

	// k0s reports "peer not found" for a different URL than member-list has: the
	// member is still there, so reset warns.
	t.Run("DirectLeavePeerNotFoundForOtherAddress", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return `{"members":{"node-a":"https://10.0.0.1:2380","node-b":"https://10.0.0.2:2380"}}`, nil
				}
				if len(args) == 2 && args[0] == "etcd" && args[1] == "leave" {
					return "", errK0sPeerNotFound("https://10.0.0.9:2380")
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}
		removed, warning := h.leaveEtcdCluster(context.Background())

		require.False(t, removed)
		assert.Contains(t, warning, "k0s etcd leave --peer-address 10.0.0.1")
	})

	// k0s binary missing: membership is unknown, so reset warns.
	t.Run("BinaryMissing", func(t *testing.T) {
		f := &fakeHelpers{}
		installFakeHelpers(t, f)
		k0sBinPath = filepath.Join(t.TempDir(), "k0s-does-not-exist")

		h := &hostInfo{Hostname: "node-a"}
		removed, warning := h.leaveEtcdCluster(context.Background())

		assert.False(t, removed, "missing binary means removal was never confirmed")
		assert.Contains(t, warning, "node-a")
	})

	// Cancelling the reset's context stops in-flight etcd calls right away.
	t.Run("RespectsParentContext", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)
		orig := etcdCmdTimeout
		etcdCmdTimeout = 2 * time.Second
		t.Cleanup(func() { etcdCmdTimeout = orig })

		f := &fakeHelpers{blockOn: "member-list"}
		installFakeHelpers(t, f)

		h := &hostInfo{Hostname: "node-a"}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		done := make(chan struct{})
		var removed bool
		var warning string
		go func() {
			removed, warning = h.leaveEtcdCluster(ctx)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("leaveEtcdCluster did not return promptly on an already-cancelled parent context — runEtcdCommand is not threading it through")
		}

		require.False(t, removed)
		assert.Contains(t, warning, "node-a")
	})

	// Progress is logged at the start and on each retry, so a long wait doesn't
	// look like a hang.
	t.Run("LogsProgressDuringRetries", func(t *testing.T) {
		speedUpEtcdLeaveRetries(t)

		f := &fakeHelpers{
			runCommandFn: func(bin string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
					return "", errors.New("connection refused")
				}
				return "", nil
			},
		}
		installFakeHelpers(t, f)

		var buf bytes.Buffer
		setLogrusOutput(t, &buf)

		h := &hostInfo{Hostname: "node-a"}
		removed, _ := h.leaveEtcdCluster(context.Background())

		require.False(t, removed)
		logged := buf.String()
		assert.NotEmpty(t, logged, "should log progress while retrying etcd member-list instead of staying silent")
		assert.GreaterOrEqual(t, strings.Count(logged, "member-list"), 2,
			"should log at least once per member-list retry attempt, not just once at the end")
	})
}

// errK0sPeerNotFound is what `k0s etcd leave` returns, via
// RunCommandWithOptions, when no etcd member has its peer URL.
func errK0sPeerNotFound(peerURL string) error {
	return fmt.Errorf("exit status 1: Error: peer not found: %s", peerURL)
}

// errEtcdMemberNotFound is etcd's MemberRemove error, surfaced by k0s.
var errEtcdMemberNotFound = errors.New("exit status 1: Error: etcdserver: member not found")

func TestStaleEtcdMemberWarning(t *testing.T) {
	// With no known peer address, the warning says how to find it instead of
	// printing the hostname as one.
	t.Run("UnknownPeerAddressDoesNotFabricateRemediationCommand", func(t *testing.T) {
		const hostname = "node-a"
		warning := staleEtcdMemberWarning(hostname, "", errors.New("k0s binary not found"))

		assert.Contains(t, warning, hostname, "the warning should still identify the node by its hostname")
		assert.NotContains(t, warning, "--peer-address "+hostname,
			"must not print a remediation command asserting the hostname is the etcd peer address when it was never confirmed")
		assert.Contains(t, warning, "member-list",
			"should tell the operator how to find the real peer address instead")
	})

	// With a known peer address, the remediation command uses it.
	t.Run("KnownPeerAddressStillUsesIt", func(t *testing.T) {
		warning := staleEtcdMemberWarning("node-a", "10.0.12.34", errors.New("etcdserver: server stopped"))

		assert.Contains(t, warning, "k0s etcd leave --peer-address 10.0.12.34")
	})

	// Line breaks in hostname or peer address can't forge extra log lines.
	t.Run("SanitizesEmbeddedNewlines", func(t *testing.T) {
		baseline := staleEtcdMemberWarning("node-a", "10.0.12.34", errors.New("etcdserver: server stopped"))
		baselineLines := strings.Count(baseline, "\n")

		injectedHostname := "node-a\nFAKE: reset already completed successfully"
		injectedPeer := "10.0.12.34\nFAKE: reset already completed successfully"
		injected := staleEtcdMemberWarning(injectedHostname, injectedPeer, errors.New("etcdserver: server stopped"))

		assert.Equal(t, baselineLines, strings.Count(injected, "\n"),
			"a newline embedded in hostname/peerAddress must not add a forged line to the warning")
		assert.NotContains(t, injected, "\nFAKE",
			"no attacker-controlled line break should survive into the logged warning")
	})

	// Line breaks in the cause (which can carry raw k0s output) can't forge extra
	// log lines.
	t.Run("SanitizesEmbeddedNewlinesInCause", func(t *testing.T) {
		baseline := staleEtcdMemberWarning("node-a", "10.0.12.34", errors.New("etcdserver: server stopped"))
		baselineLines := strings.Count(baseline, "\n")

		injectedCause := errors.New("etcdserver: server stopped\nFAKE: reset already completed successfully")
		injected := staleEtcdMemberWarning("node-a", "10.0.12.34", injectedCause)

		assert.Equal(t, baselineLines, strings.Count(injected, "\n"),
			"a newline embedded in cause must not add a forged line to the warning")
		assert.NotContains(t, injected, "\nFAKE",
			"no attacker-controlled line break in cause should survive into the logged warning")
	})
}

// TestEtcdPeerHost covers parsing a member-list peer URL, including the
// fallback to the raw value when it can't be parsed.
func TestEtcdPeerHost(t *testing.T) {
	tests := []struct {
		name     string
		peerURL  string
		expected string
	}{
		{name: "well-formed URL", peerURL: "https://10.0.12.34:2380", expected: "10.0.12.34"},
		{name: "unparseable string falls back to the raw value", peerURL: "://not a url", expected: "://not a url"},
		{name: "empty string falls back to the raw value", peerURL: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, etcdPeerHost(tt.peerURL))
		})
	}
}

// speedUpEtcdCmdTimeout shortens the k0s etcd command timeout for the
// duration of the test.
func speedUpEtcdCmdTimeout(t *testing.T) {
	t.Helper()
	orig := etcdCmdTimeout
	etcdCmdTimeout = 100 * time.Millisecond
	t.Cleanup(func() { etcdCmdTimeout = orig })
}

func TestEtcdMemberAlreadyGone(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		peerURL string
		want    bool
	}{
		{"peer not found, no URL known", errK0sPeerNotFound("https://node-a:2380"), "", false},
		{"peer not found, URL is a prefix", errK0sPeerNotFound("https://10.0.0.12:2380"), "https://10.0.0.1", false},
		// Verbatim from a live `k0s etcd leave` on CMX: a log line, then the error.
		{"real k0s stderr", errors.New(`exit status 1: time="2026-10-16 04:13:47" level=error msg="Failed to get peer name" peerURL="https://10.0.0.99:2380"` + "\n" + `Error: peer not found: https://10.0.0.99:2380`), "https://10.0.0.99:2380", true},
		{"peer not found, same URL", errK0sPeerNotFound("https://node-a:2380"), "https://node-a:2380", true},
		{"peer not found, other URL", errK0sPeerNotFound("https://node-x:2380"), "https://node-a:2380", false},
		{"etcd member not found", errEtcdMemberNotFound, "https://node-a:2380", true},
		{"unrelated error", errors.New("connection refused"), "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, etcdMemberAlreadyGone(tt.err, tt.peerURL))
		})
	}
}

// TestWarnIfEtcdLeaveNotConfirmed covers reset's call site: the warning is
// logged only when removal wasn't confirmed.
func TestWarnIfEtcdLeaveNotConfirmed(t *testing.T) {
	tests := []struct {
		name    string
		removed bool
		warning string
		wantLog bool
	}{
		{name: "not removed logs the warning", removed: false, warning: "stale etcd member for node-a", wantLog: true},
		{name: "removed logs nothing", removed: true, warning: "", wantLog: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			setLogrusOutput(t, &buf)

			warnIfEtcdLeaveNotConfirmed(tt.removed, tt.warning)

			if tt.wantLog {
				assert.Contains(t, buf.String(), tt.warning)
			} else {
				assert.Empty(t, buf.String())
			}
		})
	}
}

// useK0sControllerFiles points reset at a temp controller unit and k0s config.
// An empty config or unit means that file doesn't exist.
func useK0sControllerFiles(t *testing.T, unit bool, config string) {
	dir := t.TempDir()
	origUnit, origConfig := k0sControllerUnitPath, k0sConfigPath
	k0sControllerUnitPath = filepath.Join(dir, "k0scontroller.service")
	k0sConfigPath = filepath.Join(dir, "k0s.yaml")
	t.Cleanup(func() { k0sControllerUnitPath, k0sConfigPath = origUnit, origConfig })
	if unit {
		require.NoError(t, os.WriteFile(k0sControllerUnitPath, []byte("[Unit]\n"), 0644))
	}
	if config != "" {
		require.NoError(t, os.WriteFile(k0sConfigPath, []byte(config), 0644))
	}
}

// TestNoK0sStatusEtcdWarning covers resetting a controller where `k0s status`
// fails (e.g. k0s is stopped). Reset can't leave etcd then, and used to skip
// it without a word, leaving a stale member behind (sc-139620).
func TestNoK0sStatusEtcdWarning(t *testing.T) {
	const config = "apiVersion: k0s.k0sproject.io/v1beta1\nkind: ClusterConfig\nspec:\n  storage:\n    etcd:\n      peerAddress: 10.0.0.12\n"

	t.Run("controller with peer address in config", func(t *testing.T) {
		useK0sControllerFiles(t, true, config)
		warning, ok := noK0sStatusEtcdWarning("node-a")
		require.True(t, ok)
		assert.Contains(t, warning, "could not get k0s status on this node")
		assert.Contains(t, warning, "k0s etcd leave --peer-address 10.0.0.12")
	})

	t.Run("controller without k0s config", func(t *testing.T) {
		useK0sControllerFiles(t, true, "")
		warning, ok := noK0sStatusEtcdWarning("node-a")
		require.True(t, ok)
		assert.Contains(t, warning, `run 'k0s etcd member-list' to find "node-a"'s peer address`)
	})

	t.Run("worker", func(t *testing.T) {
		useK0sControllerFiles(t, false, config)
		_, ok := noK0sStatusEtcdWarning("node-a")
		assert.False(t, ok, "workers have no etcd member to warn about")
	})

	t.Run("unit stat fails for another reason", func(t *testing.T) {
		useK0sControllerFiles(t, false, config)
		// A path under a regular file fails with ENOTDIR, not ErrNotExist.
		k0sControllerUnitPath = filepath.Join(k0sConfigPath, "k0scontroller.service")
		warning, ok := noK0sStatusEtcdWarning("node-a")
		require.True(t, ok, "an unreadable unit path must not hide the warning")
		assert.Contains(t, warning, "k0s etcd leave --peer-address 10.0.0.12")
	})
}

// TestConfiguredEtcdPeerAddress makes sure only an explicitly configured peer
// address is used, never one filled in from k0s's defaults.
func TestConfiguredEtcdPeerAddress(t *testing.T) {
	tests := []struct {
		name   string
		config string
		want   string
	}{
		{"set", "spec:\n  storage:\n    etcd:\n      peerAddress: 10.0.0.12\n", "10.0.0.12"},
		{"not set", "spec:\n  storage:\n    type: etcd\n", ""},
		{"invalid yaml", "spec: [", ""},
		{"missing file", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useK0sControllerFiles(t, true, tt.config)
			assert.Equal(t, tt.want, configuredEtcdPeerAddress())
		})
	}
}
