package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/replicatedhq/embedded-cluster/e2e/docker"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
)

func TestPreflights(t *testing.T) {
	t.Parallel()

	cli := docker.NewCLI(t)

	container := docker.NewContainer(t).
		WithImage("debian:bookworm-slim").
		WithECBinary()
	if licensePath := os.Getenv("LICENSE_PATH"); licensePath != "" {
		t.Logf("using license %s", licensePath)
		container = container.WithLicense(licensePath)
	}
	container.Start(cli)

	t.Cleanup(func() {
		container.Destroy(cli)
	})

	_, stderr, err := container.Exec(cli,
		"apt-get update && apt-get install -y apt-utils kmod",
	)
	if err != nil {
		t.Fatalf("failed to install deps: err=%v, stderr=%s", err, stderr)
	}

	runCmd := fmt.Sprintf("%s install run-preflights --no-prompt", container.GetECBinaryPath())
	if os.Getenv("LICENSE_PATH") != "" {
		runCmd = fmt.Sprintf("%s --license %s", runCmd, container.GetLicensePath())
	}

	// we are more interested in the results
	runStdout, runStderr, runErr := container.Exec(cli, runCmd)

	stdout, stderr, err := container.Exec(cli,
		"cat /var/lib/embedded-cluster/support/host-preflight-results.json",
	)
	if err != nil {
		t.Logf("run-preflights: err=%v, stdout=%s, stderr=%s", runErr, runStdout, runStderr)
		t.Fatalf("failed to get preflight results: err=%v, stderr=%s", err, stderr)
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
					"System Clock":                true,
					"'devices' Cgroup Controller": true,
					"Default Route":               true,
					"API Access":                  true,
					"Proxy Registry Access":       true,
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
				expected := map[string]bool{}
				for _, res := range results.Warn {
					if !expected[res.Title] {
						t.Errorf("unexpected warning: %q, %q", res.Title, res.Message)
					} else {
						t.Logf("found expected warning: %q, %q", res.Title, res.Message)
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
