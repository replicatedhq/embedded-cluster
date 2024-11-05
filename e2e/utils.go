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

var commandOutputRegex = regexp.MustCompile(`{"command":"[^"]*"}`)

type nodeJoinResponse struct {
	Command string `json:"command"`
}

// findJoinCommandInOutput parses the output of the playwright.sh script and returns the join command.
func findJoinCommandInOutput(stdout string) (string, error) {
	output := commandOutputRegex.FindString(stdout)
	if output == "" {
		return "", fmt.Errorf("failed to find the join command in the output: %s", stdout)
	}
	var r nodeJoinResponse
	if err := json.Unmarshal([]byte(output), &r); err != nil {
		return "", fmt.Errorf("failed to parse node join response: %v", err)
	}
	// trim down the "./" and the "sudo" command as those are not needed. we run as
	// root and the embedded-cluster binary is on the PATH.
	command := strings.TrimPrefix(r.Command, "sudo ./")
	// replace the airgap bundle path (if any) with the local path.
	command = strings.ReplaceAll(command, "embedded-cluster.airgap", "/assets/release.airgap")
	return command, nil
}

func injectString(original, injection, after string) string {
	// Split the string into parts using the 'after' substring
	parts := strings.SplitN(original, after, 2)
	if len(parts) < 2 {
		// If 'after' substring is not found, return the original string
		return original
	}
	// Construct the new string by adding the injection between the parts
	return parts[0] + after + " " + injection + parts[1]
}

func k8sVersion() string {
	// split the version string (like 'v1.29.6+k0s.0') into the k8s version and the k0s revision
	verParts := strings.Split(os.Getenv("EXPECT_K0S_VERSION"), "+")
	if len(verParts) < 2 {
		panic(fmt.Sprintf("failed to parse k8s version %q", os.Getenv("EXPECT_K0S_VERSION")))
	}
	return verParts[0]
}

func k8sVersionPrevious() string {
	// split the version string (like 'v1.29.6+k0s.0') into the k8s version and the k0s revision
	verParts := strings.Split(os.Getenv("EXPECT_K0S_VERSION_PREVIOUS"), "+")
	if len(verParts) < 2 {
		panic(fmt.Sprintf("failed to parse previous k8s version %q", os.Getenv("EXPECT_K0S_VERSION_PREVIOUS")))
	}
	return verParts[0]
}

func k8sVersionPreviousStable() string {
	// split the version string (like 'v1.29.6+k0s.0') into the k8s version and the k0s revision
	verParts := strings.Split(os.Getenv("EXPECT_K0S_VERSION_PREVIOUS_STABLE"), "+")
	if len(verParts) < 2 {
		panic(fmt.Sprintf("failed to parse previous stable k8s version %q", os.Getenv("EXPECT_K0S_VERSION_PREVIOUS_STABLE")))
	}
	return verParts[0]
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
