package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func RequireEnvVars(t *testing.T, envVars []string) {
	for _, envVar := range envVars {
		if os.Getenv(envVar) == "" {
			t.Fatalf("missing required environment variable: %s", envVar)
		}
	}
}

var commandsOutputRegex = regexp.MustCompile(`{"commands":\[.*?\]}`)
var commandOutputRegex = regexp.MustCompile(`{"command":"[^"]*"}`) // legacy / old versions

// findJoinCommandsInOutput parses the output of the playwright.sh script and returns the join commands.
func findJoinCommandsInOutput(stdout string) ([]string, error) {
	output := commandsOutputRegex.FindString(stdout)
	if output == "" {
		return nil, fmt.Errorf("failed to find the join commands in the output: %s", stdout)
	}
	var r struct {
		Commands []string `json:"commands"`
	}
	if err := json.Unmarshal([]byte(output), &r); err != nil {
		return nil, fmt.Errorf("failed to parse node join response: %v", err)
	}
	for i := range r.Commands {
		// trim down the "sudo" command as we are running as root
		r.Commands[i] = strings.Replace(r.Commands[i], "sudo ", "", 1)
	}
	return r.Commands, nil
}

// findJoinCommandInOutput parses the output of the playwright.sh script and returns the join command.
func findJoinCommandInOutput(stdout string) (string, error) {
	output := commandOutputRegex.FindString(stdout)
	if output == "" {
		return "", fmt.Errorf("failed to find the join command in the output: %s", stdout)
	}
	var r struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(output), &r); err != nil {
		return "", fmt.Errorf("failed to parse node join response: %v", err)
	}
	// trim down the "./" and the "sudo" command as those are not needed. we run as
	// root and the embedded-cluster binary is on the PATH.
	command := strings.Replace(r.Command, "sudo ./", "", 1)
	// replace the airgap bundle path (if any) with the local path.
	command = strings.Replace(command, "embedded-cluster.airgap", "/assets/release.airgap", 1)
	return command, nil
}

func k8sVersion() string {
	// split the version string (like 'v1.29.6+k0s.0') into the k8s version and the k0s revision
	verParts := strings.Split(os.Getenv("EXPECT_K0S_VERSION"), "+")
	if len(verParts) < 2 {
		panic(fmt.Sprintf("failed to parse k8s version %q", os.Getenv("EXPECT_K0S_VERSION")))
	}
	return verParts[0]
}

func k8sVersionPrevious(n int) string {
	if n < 1 {
		panic(fmt.Sprintf("previous k0s version index must be at least 1, got %d", n))
	}
	// split the version string (like 'v1.29.6+k0s.0') into the k8s version and the k0s revision
	value := os.Getenv(fmt.Sprintf("EXPECT_K0S_VERSION_PREVIOUS_%d", n))
	if value == "" {
		panic(fmt.Sprintf("missing previous k0s version %d", n))
	}
	verParts := strings.Split(value, "+")
	if len(verParts) < 2 {
		panic(fmt.Sprintf("failed to parse previous k8s version %q", value))
	}
	return verParts[0]
}

func k8sVersionPreviousStable() string {
	// split the version string (like 'v1.29.6+k0s.0') into the k8s version and the k0s revision
	value := os.Getenv("EXPECT_K0S_VERSION_PREVIOUS_STABLE")
	if value == "" {
		panic("missing previous stable k0s version")
	}
	verParts := strings.Split(value, "+")
	if len(verParts) < 2 {
		panic(fmt.Sprintf("failed to parse previous stable k8s version %q", value))
	}
	return verParts[0]
}

func ecUpgradeTargetVersion() string {
	if os.Getenv("EXPECT_EMBEDDED_CLUSTER_UPGRADE_TARGET_VERSION") != "" {
		return os.Getenv("EXPECT_EMBEDDED_CLUSTER_UPGRADE_TARGET_VERSION") // use the env var if set
	}
	return "-upgrade" // default to requiring an upgrade suffix
}

func runInParallel(t *testing.T, fns ...func(t *testing.T) error) {
	runInParallelOffset(t, time.Duration(0), fns...)
}

func runInParallelOffset(t *testing.T, offset time.Duration, fns ...func(t *testing.T) error) {
	t.Helper()
	errCh := make(chan error, len(fns))
	for idx, fn := range fns {
		go func(fn func(t *testing.T) error) {
			time.Sleep(offset * time.Duration(idx))
			errCh <- fn(t)
		}(fn)
	}
	for range fns {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}
