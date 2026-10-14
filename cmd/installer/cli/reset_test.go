package cli

import (
	"bytes"
	"context"
	"errors"
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
	removed, warning := h.leaveEtcdCluster(context.Background())

	assert.True(t, removed)
	assert.Empty(t, warning)
	assert.Equal(t, 0, f.ranCount("etcd leave"), "should not attempt to leave when already the sole member")
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
	removed, warning := h.leaveEtcdCluster(context.Background())

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
	removed, warning := h.leaveEtcdCluster(context.Background())

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
	removed, warning := h.leaveEtcdCluster(context.Background())

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
	removed, warning := h.leaveEtcdCluster(context.Background())

	require.False(t, removed)
	assert.Contains(t, warning, "node-a")
	assert.Contains(t, warning, "member-list",
		"no peer address was ever resolved, so the warning must point at how to find the real one")
	assert.Equal(t, 1, f.ranCount("etcd leave"), "the defensive leave attempt should not be retried")
}

// TestLeaveEtcdCluster_MemberListKeyIsHostname proves the assumption
// leaveEtcdCluster's member-list lookup rests on: that `k0s etcd
// member-list`'s JSON response is keyed by this node's hostname. Verified
// against k0sproject/k0s v1.36.5-0.20260918181138-c37fe960bdee, the version
// pinned in go.mod:
//   - pkg/component/controller/etcd.go:160-166 sets etcd's --name flag to
//     os.Hostname() unless overridden by the "name" extraArg in the k0s
//     config.
//   - pkg/etcd/client.go:122-127 copies that member's raw Name field
//     unchanged into etcd.Member.Name.
//   - cmd/etcd/list.go:53-61 builds the `member-list` JSON response by
//     keying its "members" map with member.Name.
//
// So for a node that has not overridden the etcd name extraArg, its own
// member-list entry is keyed by exactly its hostname -- including a
// realistic FQDN-style hostname, not just a short placeholder name.
func TestLeaveEtcdCluster_MemberListKeyIsHostname(t *testing.T) {
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
}

// TestLeaveEtcdCluster_NeverPassesPeerAddress pins the quorum-safety
// invariant sc-139620 requires: leaveEtcdCluster must never be able to
// remove any etcd member but its own. k0sproject/k0s cmd/etcd/leave.go:40-49
// shows that `k0s etcd leave` defaults to removing the LOCAL node's own
// configured peer address; only an explicit --peer-address flag can target a
// different member. leaveEtcdCluster only ever calls runEtcdCommand("leave")
// with no extra arguments, so every `etcd leave` invocation it makes -- on
// both the direct and the defensive path -- must omit --peer-address,
// making it structurally impossible for this code to tear down another
// controller's membership and break quorum.
func TestLeaveEtcdCluster_NeverPassesPeerAddress(t *testing.T) {
	scenarios := []struct {
		name       string
		memberList string
		leaveErr   error
	}{
		{
			name:       "direct leave",
			memberList: `{"members":{"node-a":"https://node-a:2380","node-b":"https://node-b:2380"}}`,
		},
		{
			name:       "defensive leave",
			memberList: `{"members":{"node-b":"https://node-b:2380","node-c":"https://node-c:2380"}}`,
			leaveErr:   errors.New("etcdserver: member not found"),
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			speedUpEtcdLeaveRetries(t)
			var leaveArgs [][]string
			f := &fakeHelpers{
				runCommandFn: func(bin string, args ...string) (string, error) {
					if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
						return sc.memberList, nil
					}
					if len(args) == 2 && args[0] == "etcd" && args[1] == "leave" {
						leaveArgs = append(leaveArgs, append([]string(nil), args...))
						return "", sc.leaveErr
					}
					return "", nil
				},
			}
			installFakeHelpers(t, f)

			h := &hostInfo{Hostname: "node-a"}
			h.leaveEtcdCluster(context.Background())

			require.NotEmpty(t, leaveArgs, "expected at least one etcd leave call")
			for _, args := range leaveArgs {
				assert.NotContains(t, args, "--peer-address",
					"leaveEtcdCluster must never pass --peer-address, or it could target a member other than itself")
			}
		})
	}
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
	removed, warning := h.leaveEtcdCluster(context.Background())

	require.False(t, removed)
	assert.Contains(t, warning, "node-a")
	assert.Contains(t, warning, "member-list",
		"no peer address was ever resolved, so the warning must point at how to find the real one")
	assert.Equal(t, 0, f.ranCount("etcd leave"), "should not attempt to leave without a confirmed member list")
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
	removed, warning := h.leaveEtcdCluster(context.Background())

	require.False(t, removed)
	assert.Contains(t, warning, "node-a")
	assert.Contains(t, warning, "k0s etcd leave --peer-address node-a")
}

// TestLeaveEtcdCluster_RemediationUsesEtcdPeerAddressNotHostname covers the
// review finding on emc-r1lp (PR #4019): leaveEtcdCluster's remediation
// command told a human to run `k0s etcd leave --peer-address <hostname>`, but
// `k0s etcd leave --peer-address` ultimately resolves the peer by its etcd
// peer address (k0sproject/k0s cmd/etcd/leave.go:40-56), not by the
// member-list key (which is always the node's os.Hostname()). On a node that
// peers over an address other than its DNS hostname (e.g. a private IP),
// those two values differ -- and the member list's own peer address, which
// this function already has in scope when it builds the remediation message,
// must be used instead of guessing from h.Hostname.
func TestLeaveEtcdCluster_RemediationUsesEtcdPeerAddressNotHostname(t *testing.T) {
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
}

// TestStaleEtcdMemberWarning_UnknownPeerAddressDoesNotFabricateRemediationCommand
// covers the review finding on emc-r1lp (PR #4019): when no real peer
// address was ever obtained (k0s binary missing, member-list call failed, or
// this node's own key was already absent), falling back to the hostname and
// printing it as `--peer-address <hostname>` hands the operator a command
// that may not work -- `k0s etcd leave --peer-address` resolves peers by
// their etcd peer address, not by hostname, and the two can differ. The
// warning must tell the operator to look up the real peer address instead of
// asserting a fabricated one.
func TestStaleEtcdMemberWarning_UnknownPeerAddressDoesNotFabricateRemediationCommand(t *testing.T) {
	const hostname = "node-a"
	warning := staleEtcdMemberWarning(hostname, "", errors.New("k0s binary not found"))

	assert.Contains(t, warning, hostname, "the warning should still identify the node by its hostname")
	assert.NotContains(t, warning, "--peer-address "+hostname,
		"must not print a remediation command asserting the hostname is the etcd peer address when it was never confirmed")
	assert.Contains(t, warning, "member-list",
		"should tell the operator how to find the real peer address instead")
}

// TestStaleEtcdMemberWarning_KnownPeerAddressStillUsesIt pins the unchanged
// behavior for the case this fix must not touch: when a real peer address
// was resolved from the member list, the remediation command must keep using
// it directly.
func TestStaleEtcdMemberWarning_KnownPeerAddressStillUsesIt(t *testing.T) {
	warning := staleEtcdMemberWarning("node-a", "10.0.12.34", errors.New("etcdserver: server stopped"))

	assert.Contains(t, warning, "k0s etcd leave --peer-address 10.0.12.34")
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
	removed, warning := h.leaveEtcdCluster(context.Background())

	require.False(t, removed)
	assert.Contains(t, warning, "node-a")
	assert.Contains(t, warning, "k0s etcd leave --peer-address node-a")
}

// TestLeaveEtcdCluster_DirectLeaveMemberNotFound covers the direct leave path
// (this node is still listed in member-list) racing a removal that already
// completed by the time the leave call reaches etcd. A "member not found"
// error here is the same definitive confirmation the defensive leave path
// already trusts (TestLeaveEtcdCluster_AlreadyRemoved) — it must not be
// treated as just another failed attempt to retry and eventually warn about.
func TestLeaveEtcdCluster_DirectLeaveMemberNotFound(t *testing.T) {
	speedUpEtcdLeaveRetries(t)
	f := &fakeHelpers{
		runCommandFn: func(bin string, args ...string) (string, error) {
			if len(args) == 2 && args[0] == "etcd" && args[1] == "member-list" {
				return `{"members":{"node-a":"https://node-a:2380","node-b":"https://node-b:2380"}}`, nil
			}
			if len(args) == 2 && args[0] == "etcd" && args[1] == "leave" {
				return "", errors.New("etcdserver: member not found")
			}
			return "", nil
		},
	}
	installFakeHelpers(t, f)

	h := &hostInfo{Hostname: "node-a"}
	removed, warning := h.leaveEtcdCluster(context.Background())

	assert.True(t, removed, "a definitive 'member not found' confirms removal, it is not a failure")
	assert.Empty(t, warning)
	assert.Equal(t, 1, f.ranCount("etcd leave"), "should not retry once 'member not found' confirms removal")
}

// TestLeaveEtcdCluster_BinaryMissing covers resetting a node where the k0s
// binary is already gone (e.g. a partially-completed install). leaveEtcdCluster
// cannot run `k0s etcd leave` at all here, so it has no evidence either way
// about this node's etcd membership — it must report that as unknown rather
// than claiming removal was confirmed.
func TestLeaveEtcdCluster_BinaryMissing(t *testing.T) {
	f := &fakeHelpers{}
	installFakeHelpers(t, f)
	k0sBinPath = filepath.Join(t.TempDir(), "k0s-does-not-exist")

	h := &hostInfo{Hostname: "node-a"}
	removed, warning := h.leaveEtcdCluster(context.Background())

	assert.False(t, removed, "missing binary means removal was never confirmed")
	assert.Contains(t, warning, "node-a")
}

// TestLeaveEtcdCluster_RespectsParentContext covers LOW-2 from the PR #4019
// review (round 2): runEtcdCommand used to derive its timeout from
// context.Background(), so cancelling the reset's own context had no effect
// on an in-flight etcd call -- it would keep running until etcdCmdTimeout
// fired on its own. Cancelling the parent before the call must stop the
// member-list retry loop immediately instead of waiting out that separate
// timeout.
func TestLeaveEtcdCluster_RespectsParentContext(t *testing.T) {
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
}

// TestWarnIfEtcdLeaveNotConfirmed covers LOW-7 from the PR #4019 review
// (round 2): ResetCmd's etcd-leave call site (reset.go:~172) only logs
// leaveEtcdCluster's warning when removal was not confirmed, pulled out into
// warnIfEtcdLeaveNotConfirmed so that exact call-site behavior is testable
// directly.
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
			origOut := logrus.StandardLogger().Out
			logrus.SetOutput(&buf)
			t.Cleanup(func() { logrus.SetOutput(origOut) })

			warnIfEtcdLeaveNotConfirmed(tt.removed, tt.warning)

			if tt.wantLog {
				assert.Contains(t, buf.String(), tt.warning)
			} else {
				assert.Empty(t, buf.String())
			}
		})
	}
}
