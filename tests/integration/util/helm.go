package util

import (
	"os/exec"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	helmcli "helm.sh/helm/v3/pkg/cli"
)

func HelmClient(t *testing.T, kubeconfig string) helm.Client {
	envSettings := helmcli.New()
	envSettings.KubeConfig = kubeconfig

	hcli, err := helm.NewClient(helm.HelmOptions{
		HelmPath:              "helm", // use the helm binary in PATH
		KubernetesEnvSettings: envSettings,
	})
	if err != nil {
		t.Fatalf("failed to create helm client: %s", err)
	}
	t.Cleanup(func() {
		hcli.Close()
	})
	return hcli
}

func WriteHelmValuesFile(t *testing.T, values string) string {
	return WriteTempFile(t, "values-*.yaml", []byte(values), 0644)
}

func AddHelmRepo(t *testing.T, name string, url string) {
	t.Logf("adding helm repo %s", url)
	cmd := exec.Command("helm", "repo", "add", name, url)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("output: %s", out)
		t.Fatalf("failed to add helm repo: %s", err)
	}

	cmd = exec.Command("helm", "repo", "update")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Logf("output: %s", out)
		t.Fatalf("failed to update helm repos: %s", err)
	}
	t.Logf("helm repo %s added", url)
}

func HelmInstall(t *testing.T, kubeconfig string, namespace string, name string, version string, chart string, values string) {
	t.Logf("installing helm chart %s:%s", name, version)
	cmd := exec.Command(
		"helm", "install",
		"--kubeconfig", kubeconfig,
		"--namespace", namespace,
		name, chart,
		"--version", version, "--values", values,
		"--create-namespace",
		"--atomic", "--wait", "--wait-for-jobs",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("output: %s", out)
		t.Fatalf("failed to install helm chart: %s", err)
	}
	t.Logf("helm chart %s:%s installed", name, version)
}
