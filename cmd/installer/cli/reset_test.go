package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
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

// TestLeaveEtcdCluster_SoleMember covers the legitimate no-op case: this node
// is the only etcd member, so there is nothing to leave.
func TestLeaveEtcdCluster_SoleMember(t *testing.T) {
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
	removed, warning := h.leaveEtcdCluster()

	assert.True(t, removed)
	assert.Empty(t, warning)
	assert.False(t, f.ran("k0s etcd leave"), "should not attempt to leave when already the sole member")
}

// TestLeaveEtcdCluster_Success covers the happy path: member-list succeeds,
// there is more than one member, and the leave call succeeds.
func TestLeaveEtcdCluster_Success(t *testing.T) {
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
	removed, warning := h.leaveEtcdCluster()

	assert.True(t, removed)
	assert.Empty(t, warning)
	assert.Equal(t, 1, f.ranCount("etcd leave"), "should actually attempt to leave the cluster")
}

// TestLeaveEtcdCluster_AlreadyRemoved covers re-running reset on a node whose
// etcd membership was already removed by an earlier attempt (or a manual
// `k0s etcd leave`): member-list succeeds but no longer lists this hostname.
// The member-list key is assumed to equal h.Hostname exactly; that assumption
// is unverified, so rather than trusting the absence alone, leaveEtcdCluster
// still makes one defensive leave attempt (a harmless no-op if we are truly
// already gone, but the actual removal if the key assumption was wrong). It
// must not block reset on whatever that defensive attempt returns.
func TestLeaveEtcdCluster_AlreadyRemoved(t *testing.T) {
	speedUpEtcdLeaveRetries(t)
	f := &fakeHelpers{
		runCommandFn: func(bin string, args ...string) (string, error) {
			if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
				return `{"members":{"node-b":"https://node-b:2380","node-c":"https://node-c:2380"}}`, nil
			}
			if len(args) == 2 && args[0] == "etcd" && args[1] == "leave" {
				return "", errors.New("etcdserver: member not found")
			}
			return "", nil
		},
	}
	installFakeHelpers(t, f)

	h := &hostInfo{Hostname: "node-a"}
	removed, warning := h.leaveEtcdCluster()

	assert.True(t, removed)
	assert.Empty(t, warning)
	assert.Equal(t, 1, f.ranCount("etcd leave"), "should make a defensive leave attempt rather than trusting the member-list key match alone")
}

// TestLeaveEtcdCluster_AlreadyRemovedButActuallyStillAMember covers the case
// the defensive leave attempt in TestLeaveEtcdCluster_AlreadyRemoved guards
// against: the member-list key for this node didn't match h.Hostname even
// though it is still a voting member, and the leave call that follows
// actually performs the removal. That single attempt must not be retried.
func TestLeaveEtcdCluster_AlreadyRemovedButActuallyStillAMember(t *testing.T) {
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
	removed, warning := h.leaveEtcdCluster()

	assert.True(t, removed)
	assert.Empty(t, warning)
	assert.Equal(t, 1, f.ranCount("etcd leave"), "the defensive leave attempt should not be retried")
}

// TestLeaveEtcdCluster_AlreadyRemovedButLeaveFailsForUnrelatedReason covers
// the bug flagged on review of emc-r1lp: the defensive leave attempt in
// TestLeaveEtcdCluster_AlreadyRemoved can fail for a reason other than "this
// member doesn't exist" (e.g. the etcd endpoint being unreachable), and that
// failure must not be swallowed into removed=true — doing so would silently
// reintroduce sc-139620 for exactly the node the defensive attempt exists to
// protect.
func TestLeaveEtcdCluster_AlreadyRemovedButLeaveFailsForUnrelatedReason(t *testing.T) {
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
	removed, warning := h.leaveEtcdCluster()

	require.False(t, removed)
	assert.Contains(t, warning, "node-a")
	assert.Contains(t, warning, "k0s etcd leave --peer-address node-a")
	assert.Equal(t, 1, f.ranCount("etcd leave"), "the defensive leave attempt should not be retried")
}

// TestLeaveEtcdCluster_MemberListFails is the bug from sc-139620: when
// membership cannot be determined, reset must not silently assume the node
// is already gone. It must report that a stale member may remain and how to
// remove it.
func TestLeaveEtcdCluster_MemberListFails(t *testing.T) {
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
	removed, warning := h.leaveEtcdCluster()

	require.False(t, removed)
	assert.Contains(t, warning, "node-a")
	assert.Contains(t, warning, "k0s etcd leave --peer-address node-a")
	assert.False(t, f.ran("k0s etcd leave"), "should not attempt to leave without a confirmed member list")
}

// TestLeaveEtcdCluster_EtcdStopped covers the previously-mishandled case:
// local etcd being stopped was treated as proof the membership was already
// removed, when it is unrelated information.
func TestLeaveEtcdCluster_EtcdStopped(t *testing.T) {
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
	removed, warning := h.leaveEtcdCluster()

	require.False(t, removed)
	assert.Contains(t, warning, "node-a")
	assert.Contains(t, warning, "k0s etcd leave --peer-address node-a")
}

// speedUpEtcdCmdTimeout shortens the k0s etcd command timeout for the
// duration of the test.
func speedUpEtcdCmdTimeout(t *testing.T) {
	t.Helper()
	orig := etcdCmdTimeout
	etcdCmdTimeout = 100 * time.Millisecond
	t.Cleanup(func() { etcdCmdTimeout = orig })
}

// TestLeaveEtcdCluster_MemberListBlocks reproduces the hang seen in CI: on a
// cluster with only two voting controllers, resetting both at once can leave
// neither etcd request able to reach quorum, so `k0s etcd member-list` never
// returns. It must give up instead of hanging the whole reset (observed as
// an E2E test timing out after over an hour, stuck at this exact call).
func TestLeaveEtcdCluster_MemberListBlocks(t *testing.T) {
	speedUpEtcdLeaveRetries(t)
	speedUpEtcdCmdTimeout(t)

	f := &fakeHelpers{blockOn: "member-list"}
	installFakeHelpers(t, f)

	h := &hostInfo{Hostname: "node-a"}

	done := make(chan struct{})
	var removed bool
	var warning string
	go func() {
		removed, warning = h.leaveEtcdCluster()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("leaveEtcdCluster did not return: the k0s etcd member-list call is still unbounded")
	}

	require.False(t, removed)
	assert.Contains(t, warning, "node-a")
	assert.False(t, f.ran("k0s etcd leave"), "should not attempt to leave without a confirmed member list")
}

// TestLeaveEtcdCluster_LeaveBlocks covers the same unbounded-call hang on the
// leave call itself, once membership is confirmed.
func TestLeaveEtcdCluster_LeaveBlocks(t *testing.T) {
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
		removed, warning = h.leaveEtcdCluster()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("leaveEtcdCluster did not return: the k0s etcd leave call is still unbounded")
	}

	require.False(t, removed)
	assert.Contains(t, warning, "node-a")
}

// TestLeaveEtcdCluster_LeaveFailsAfterRetries covers exhausting the leave
// retries on a persistent error: it must warn, not claim this is normal.
func TestLeaveEtcdCluster_LeaveFailsAfterRetries(t *testing.T) {
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
	removed, warning := h.leaveEtcdCluster()

	require.False(t, removed)
	assert.Contains(t, warning, "node-a")
	assert.Contains(t, warning, "k0s etcd leave --peer-address node-a")
}
