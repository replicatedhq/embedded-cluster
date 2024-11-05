package e2e

import (
	"strings"
	"testing"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/stretchr/testify/assert"
)

func TestPreflights_Sysctl(t *testing.T) {
	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	tests := []struct {
		name         string
		setupCommand []string
		expectFail   []preflights.Record
		expectPass   []preflights.Record
	}{
		{
			name: "ARP config is valid, preflights pass",
			setupCommand: []string{
				`
					sysctl net.ipv4.conf.default.arp_filter=0 && \
					sysctl net.ipv4.conf.default.arp_ignore=0 && \
					sysctl net.ipv4.conf.all.arp_filter=0 && \
					sysctl net.ipv4.conf.all.arp_ignore=0
				`,
			},
			expectPass: []preflights.Record{
				{
					Title:   "ARP filtering is not enabled by default for new interfaces",
					Message: "ARP filtering is not enabled by default for newly created interfaces on the host.",
				},
				{
					Title:   "ARP ignore is not enabled by default for new interfaces",
					Message: "ARP ignore is not enabled by default for newly created interfaces on the host.",
				},
				{
					Title:   "ARP filtering is not enabled for all interfaces",
					Message: "ARP filtering is not enabled for all interfaces on the host.",
				},
				{
					Title:   "ARP ignore is not enabled for all interfaces",
					Message: "ARP ignore is not enabled for all interfaces interfaces on the host.",
				},
			},
		},
		{
			name: "ARP config is not valid, all arp preflights fail",
			setupCommand: []string{
				`
					sysctl net.ipv4.conf.default.arp_filter=1 && \
					sysctl net.ipv4.conf.default.arp_ignore=1 && \
					sysctl net.ipv4.conf.all.arp_filter=1 && \
					sysctl net.ipv4.conf.all.arp_ignore=8
				`,
			},
			expectFail: []preflights.Record{
				{
					Title:   "ARP filtering is not enabled by default for new interfaces",
					Message: "ARP filtering is enabled by default for newly created interfaces on the host. Disable it by running 'sysctl net.ipv4.conf.default.arp_filter=0'.",
				},
				{
					Title:   "ARP ignore is not enabled by default for new interfaces",
					Message: "ARP ignore is enabled by default for newly created interfaces on the host. Disable it by running 'sysctl net.ipv4.conf.default.arp_ignore=0'.",
				},
				{
					Title:   "ARP filtering is not enabled for all interfaces",
					Message: "ARP filtering is enabled for all interfaces on the host. Disable it by running 'sysctl net.ipv4.conf.all.arp_filter=0'.",
				},
				{
					Title:   "ARP ignore is not enabled for all interfaces",
					Message: "ARP ignore is enabled for all interfaces on the host. Disable it by running 'sysctl net.ipv4.conf.all.arp_ignore=0'.",
				},
			},
		},
	}

	runCmd := []string{"embedded-cluster install run-preflights --no-prompt --license /assets/license.yaml"}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Setup the node
			setupStdout, setupStderr, setupErr := tc.RunCommandOnNode(0, test.setupCommand)
			if setupErr != nil || setupStderr != "" {
				t.Logf("run-preflights-sysctl-setup: err=%v, stdout=%s, stderr=%s", setupErr, setupStdout, setupStderr)
				t.Fatalf("failed to get preflight results: err=%v, stderr=%s", setupErr, setupStderr)

			}

			// Run preflights
			runStdout, runStderr, runErr := tc.RunCommandOnNode(0, runCmd)
			stdout, stderr, err := tc.RunCommandOnNode(0, []string{"cat /var/lib/embedded-cluster/support/host-preflight-results.json"})
			if err != nil || stderr != "" {
				t.Logf("run-preflights-sysctl: err=%v, stdout=%s, stderr=%s", runErr, runStdout, runStderr)
				t.Fatalf("failed to get preflight results: err=%v, stderr=%s", err, stderr)
			}

			results, err := preflights.OutputFromReader(strings.NewReader(stdout))
			if err != nil {
				t.Fatalf("failed to parse preflight results: %v", err)
			}
			t.Logf("preflight-results: %s", stdout)

			for _, expectFail := range test.expectFail {
				assert.Contains(t, results.Fail, expectFail)
			}

			for _, expectPass := range test.expectPass {
				assert.Contains(t, results.Pass, expectPass)
			}
		})
	}
}

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

	runCmd := []string{"embedded-cluster install run-preflights --no-prompt --license /assets/license.yaml"}

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

	results, err := preflights.OutputFromReader(strings.NewReader(stdout))
	if err != nil {
		t.Fatalf("failed to parse preflight results: %v", err)
	}

	tests := []struct {
		name   string
		assert func(t *testing.T, results *preflights.Output)
	}{
		{
			name: "Should contain fio results",
			assert: func(t *testing.T, results *preflights.Output) {
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
			assert: func(t *testing.T, results *preflights.Output) {
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
			assert: func(t *testing.T, results *preflights.Output) {
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
			assert: func(t *testing.T, results *preflights.Output) {
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
