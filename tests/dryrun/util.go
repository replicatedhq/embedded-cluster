package dryrun

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/cli"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	dryruntypes "github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	//go:embed assets/install-release.yaml
	releaseData string

	//go:embed assets/install-license.yaml
	licenseData string
)

func dryrunInstall(t *testing.T, args ...string) dryruntypes.DryRun {
	if err := embedReleaseData(); err != nil {
		t.Fatalf("fail to embed release data: %v", err)
	}

	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	dryrun.Init(drFile, nil)

	licenseFile := filepath.Join(t.TempDir(), "license.yaml")
	require.NoError(t, os.WriteFile(licenseFile, []byte(licenseData), 0644))

	if err := runInstallerCmd(
		append([]string{
			"install",
			"--yes",
			"--license", licenseFile,
		}, args...)...,
	); err != nil {
		t.Fatalf("fail to dryrun install embedded-cluster: %v", err)
	}

	dr, err := dryrun.Load()
	if err != nil {
		t.Fatalf("fail to unmarshal dryrun output: %v", err)
	}
	return *dr
}

func dryrunUpdate(t *testing.T, args ...string) dryruntypes.DryRun {
	if err := embedReleaseData(); err != nil {
		t.Fatalf("fail to embed release data: %v", err)
	}

	if err := runInstallerCmd(
		append([]string{
			"update",
		}, args...)...,
	); err != nil {
		t.Fatalf("fail to dryrun install embedded-cluster: %v", err)
	}

	dr, err := dryrun.Load()
	if err != nil {
		t.Fatalf("fail to unmarshal dryrun output: %v", err)
	}
	return *dr
}

func embedReleaseData() error {
	if err := release.SetReleaseDataForTests(map[string][]byte{
		"release.yaml": []byte(releaseData),
	}); err != nil {
		return fmt.Errorf("set release data: %v", err)
	}
	return nil
}

func runInstallerCmd(args ...string) error {
	fullArgs := append([]string{"dryrun"}, args...)
	os.Args = fullArgs // for reporting

	installerCmd := cli.RootCmd(context.Background(), "dryrun")
	installerCmd.SetArgs(args)
	return installerCmd.Execute()
}

func readK0sConfig(t *testing.T) k0sv1beta1.ClusterConfig {
	stdout, err := exec.Command("cat", runtimeconfig.PathToK0sConfig()).Output()
	if err != nil {
		t.Fatalf("fail to get k0s config: %v", err)
	}
	k0sConfig := k0sv1beta1.ClusterConfig{}
	if err := yaml.Unmarshal(stdout, &k0sConfig); err != nil {
		t.Fatalf("fail to unmarshal k0s config: %v", err)
	}
	return k0sConfig
}

func assertCollectors(t *testing.T, actual []*troubleshootv1beta2.HostCollect, expected map[string]struct {
	match    func(*troubleshootv1beta2.HostCollect) bool
	validate func(*troubleshootv1beta2.HostCollect)
}) {
	found := make(map[string]bool)
	for _, collector := range actual {
		for name, assertion := range expected {
			if assertion.match(collector) {
				found[name] = true
				assertion.validate(collector)
			}
		}
	}
	for name := range expected {
		assert.True(t, found[name], fmt.Sprintf("%s collector not found", name))
	}
}

func assertAnalyzers(t *testing.T, actual []*troubleshootv1beta2.HostAnalyze, expected map[string]struct {
	match    func(*troubleshootv1beta2.HostAnalyze) bool
	validate func(*troubleshootv1beta2.HostAnalyze)
}) {
	found := make(map[string]bool)
	for _, collector := range actual {
		for name, assertion := range expected {
			if assertion.match(collector) {
				found[name] = true
				assertion.validate(collector)
			}
		}
	}
	for name := range expected {
		assert.True(t, found[name], fmt.Sprintf("%s collector not found", name))
	}
}

func assertMetrics(t *testing.T, actual []dryruntypes.Metric, expected []struct {
	title    string
	validate func(string)
}) {
	if len(actual) != len(expected) {
		t.Errorf("expected %d metrics, got %d", len(expected), len(actual))
		return
	}
	for i, exp := range expected {
		m := actual[i]
		if m.Title != exp.title {
			t.Errorf("expected metric %s at position %d, got %s", exp.title, i, m.Title)
			continue
		}
		exp.validate(m.Payload)
	}
}

func assertEnv(t *testing.T, actual, expected map[string]string) {
	for expectedKey, expectedValue := range expected {
		assert.Equal(t, expectedValue, actual[expectedKey])
	}
}

// assertCommands asserts that the expected commands are present in the actual commands
// if assertAll is true, it will fail the test if any command is present in the actual commands that was not expected
func assertCommands(t *testing.T, actual []dryruntypes.Command, expected []interface{}, assertAll bool) {
	for _, exp := range expected {
		found := false
		for i, a := range actual {
			switch c := exp.(type) {
			case string:
				if strings.Contains(a.Cmd, c) {
					found = true
					actual = append(actual[:i], actual[i+1:]...)
					break
				}
			case *regexp.Regexp:
				if c.MatchString(a.Cmd) {
					found = true
					actual = append(actual[:i], actual[i+1:]...)
					break
				}
			default:
				t.Fatalf("unexpected command type %T", c)
			}
		}
		if !found {
			t.Errorf("expected command %v not found", exp)
		}
	}

	if assertAll && len(actual) > 0 {
		t.Errorf("unexpected commands: %v", actual)
	}
}

func assertConfigMapExists(t *testing.T, kcli client.Client, name string, namespace string) {
	var cm corev1.ConfigMap
	err := kcli.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &cm)
	assert.NoError(t, err, "failed to get configmap %s in namespace %s", name, namespace)
}

func assertSecretExists(t *testing.T, kcli client.Client, name string, namespace string) {
	var secret corev1.Secret
	err := kcli.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &secret)
	assert.NoError(t, err, "failed to get secret %s in namespace %s", name, namespace)
}

func assertHelmValues(
	t *testing.T,
	k0sConfig k0sv1beta1.ClusterConfig,
	chartName string,
	expectedValues map[string]interface{},
) {
	actualValues := map[string]interface{}{}
	for _, ext := range k0sConfig.Spec.Extensions.Helm.Charts {
		if ext.Name == chartName {
			if err := yaml.Unmarshal([]byte(ext.Values), &actualValues); err != nil {
				t.Fatalf("fail to unmarshal %s helm values: %v", chartName, err)
			}
		}
	}
	for expectedKey, expectedValue := range expectedValues {
		actualValue, err := helm.GetValue(actualValues, expectedKey)
		assert.NoError(t, err)
		assert.Equal(t, expectedValue, actualValue)
	}
}
