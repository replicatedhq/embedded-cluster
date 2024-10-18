package util

import (
	"os"
	"os/exec"
	"testing"
)

func WriteHelmValuesFile(t *testing.T, name string, values string) string {
	f, err := os.CreateTemp("", "values-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %s", err)
	}
	f.Close()

	err = os.WriteFile(f.Name(), []byte(values), 0644)
	if err != nil {
		t.Fatalf("failed to write values to file: %s", err)
	}

	return f.Name()
}

func AddHelmRepo(t *testing.T, name string, url string) {
	t.Logf("adding helm repo %s", url)
	cmd := exec.Command("helm", "repo", "add", name, url)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("stdout: %s", out)
		t.Fatalf("failed to add helm repo: %s", err)
	}

	cmd = exec.Command("helm", "repo", "update")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Logf("stdout: %s", out)
		t.Fatalf("failed to update helm repos: %s", err)
	}
	t.Logf("helm repo %s added", url)
}

func HelmInstall(t *testing.T, namespace string, name string, version string, chart string, values string) {
	t.Logf("installing helm chart %s:%s", name, version)
	cmd := exec.Command(
		"helm", "install",
		"--namespace", namespace,
		name, chart,
		"--version", version, "--values", values,
		"--create-namespace",
		// "--atomic", "--wait", "--wait-for-jobs",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("stdout: %s", out)
		t.Fatalf("failed to install helm chart: %s", err)
	}
	t.Logf("helm chart %s:%s installed", name, version)
}
