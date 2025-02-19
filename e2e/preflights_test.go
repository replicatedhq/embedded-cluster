package e2e

import (
	"strings"
	"testing"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights/types"
)

func TestPreflights(t *testing.T) {
	t.Parallel()

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	_, stderr, err := tc.RunCommandOnNode(0, []string{"apt-get update && apt-get install -y apt-utils netcat-traditional"})
	if err != nil {
		t.Fatalf("failed to install deps: err=%v, stderr=%s", err, stderr)
	}

	if _, stderr, err = tc.RunCommandOnNode(0, []string{"nohup netcat -l -p 10250 &"}); err != nil {
		t.Fatalf("failed to start netcat: err=%v, stderr=%s", err, stderr)
	}

	if _, stderr, err = tc.RunCommandOnNode(0, []string{"nohup netcat -l 127.0.0.1 -p 50000 &"}); err != nil {
		t.Fatalf("failed to start netcat: err=%v, stderr=%s", err, stderr)
	}

	if _, stderr, err = tc.RunCommandOnNode(0, []string{"nohup netcat -l -u -p 4789 &"}); err != nil {
		t.Fatalf("failed to start netcat: err=%v, stderr=%s", err, stderr)
	}

	runCmd := []string{"embedded-cluster install run-preflights --yes --license /assets/license.yaml"}

	// we are more interested in the results
	runStdout, runStderr, runErr := tc.RunCommandOnNode(0, runCmd)

	stdout, stderr, err := tc.RunCommandOnNode(0, []string{"cat /var/lib/embedded-cluster/support/host-preflight-results.json"})
	if err != nil {
		t.Logf("run-preflights: err=%v, stdout=%s, stderr=%s", runErr, runStdout, runStderr)
		t.Fatalf("failed to get preflight results: err=%v, stderr=%s", err, stderr)
	}

	_, stderr, err = tc.RunCommandOnNode(0, []string{"ls /var/lib/embedded-cluster/support/preflight-bundle.tar.gz"})
	if err != nil {
		t.Logf("run-preflights: err=%v, stdout=%s, stderr=%s", runErr, runStdout, runStderr)
		t.Fatalf("failed to list preflight bundle: err=%v, stderr=%s", err, stderr)
	}

	results, err := types.OutputFromReader(strings.NewReader(stdout))
	if err != nil {
		t.Fatalf("failed to parse preflight results: %v", err)
	}

	tests := []struct {
		name   string
		assert func(t *testing.T, results *types.Output)
	}{
		{
			name: "Should contain fio results",
			assert: func(t *testing.T, results *types.Output) {
				for _, res := range results.Pass {
					if res.Title == "Filesystem Write Latency" {
						t.Logf("fio test passed: %s", res.Message)
						return
					}
				}
				for _, res := range results.Fail {
					if !strings.Contains(res.Message, "Write latency is high") {
						t.Errorf("fio test failed: %s", res.Message)
					}
					// as long as fio ran successfully, we're good
					t.Logf("fio test failed: %s", res.Message)
				}

				t.Errorf("fio test not found")
			},
		},
		{
			name: "Should not contain unexpected failures",
			assert: func(t *testing.T, results *types.Output) {
				expected := map[string]bool{
					// TODO: work to remove these
					"System Clock":                            true,
					"'devices' Cgroup Controller":             true,
					"API Access":                              true,
					"Proxy Registry Access":                   true,
					"Kubelet Port Availability":               true,
					"Calico Communication Port Availability":  true,
					"Local Artifact Mirror Port Availability": true,
					// as long as fio ran successfully, we're good
					"Filesystem Write Latency": true,
				}
				for _, res := range results.Fail {
					if !expected[res.Title] {
						t.Errorf("unexpected failure: %q, %q", res.Title, res.Message)
					} else {
						t.Logf("found expected failure: %q, %q", res.Title, res.Message)
					}
				}
			},
		},
		{
			name: "Should not contain unexpected warnings",
			assert: func(t *testing.T, results *types.Output) {
				expected := map[string]bool{
					"Default Route": true,
				}
				for _, res := range results.Warn {
					if !expected[res.Title] {
						t.Errorf("unexpected warning: %q, %q", res.Title, res.Message)
					} else {
						t.Logf("found expected warning: %q, %q", res.Title, res.Message)
					}
				}
			},
		},
		{
			name: "Should contain port failures",
			assert: func(t *testing.T, results *types.Output) {
				expected := map[string]bool{
					"Kubelet Port Availability":               false,
					"Calico Communication Port Availability":  false,
					"Local Artifact Mirror Port Availability": false,
				}
				for _, res := range results.Fail {
					if _, ok := expected[res.Title]; ok {
						expected[res.Title] = true
					}
				}
				for title, found := range expected {
					if !found {
						t.Errorf("expected port failure not found: %q", title)
					}
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, results)
		})
	}
}

func TestPreflightsNoexec(t *testing.T) {
	t.Parallel()

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	_, stderr, err := tc.RunCommandOnNode(0, []string{"mkdir -p /var/lib/ec && mkdir -p /var/lib/ecmount && mount --bind -o defaults,bind,noexec /var/lib/ec /var/lib/ecmount"})
	if err != nil {
		t.Fatalf("failed to install deps: err=%v, stderr=%s", err, stderr)
	}

	runCmd := []string{"embedded-cluster install run-preflights --yes --license /assets/license.yaml --data-dir /var/lib/ecmount"}

	// we are more interested in the results
	runStdout, _, runErr := tc.RunCommandOnNode(0, runCmd)
	if runErr == nil {
		t.Fatalf("expected error, got nil")
	}

	if !strings.Contains(runStdout, "Please make sure that the filesystem at /var/lib/ecmount is executable.") {
		t.Fatalf("expected not executable error, got %q", runStdout)
	}
}
