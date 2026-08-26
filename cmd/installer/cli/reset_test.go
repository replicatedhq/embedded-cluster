package cli

import (
	"context"
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

func (f *fakeHelpers) RunCommand(bin string, args ...string) (string, error) {
	f.record(bin, args...)
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
