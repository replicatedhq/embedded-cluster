package dryrun

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/cli"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	dryruntypes "github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotscrypto "github.com/replicatedhq/kotskinds/pkg/crypto"
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

	//go:embed assets/cluster-config.yaml
	clusterConfigData string

	//go:embed assets/cluster-config-nodomains.yaml
	clusterConfigNoDomainsData string

	//go:embed assets/kotskinds-application.yaml
	applicationData string

	//go:embed assets/kotskinds-config.yaml
	configData string

	//go:embed assets/kotskinds-chart.yaml
	helmChartData string

	//go:embed assets/chart.tgz
	helmChartArchiveData string

	//go:embed assets/install-license.yaml
	licenseData string
)

// dryrunPublicKey is the public key used for test license signature verification.
// This must match the key ID 6f21b4d9865f45b8a15bd884fb4028d2 in the test license.
const dryrunPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwWEoVA/AQhzgG81k4V+C
7c7xoNKSnP8XKSkuYiCbsYyicsWxMtwExkueVKXvEa/DQm7NCDBOdFQFhFQKzKvn
Jh2rXnPZn3OyNQ9Ru+4XBi4kOa1V9g5VFSgwbBttuVtWtPZC2B4vdCVXyX4TzLYe
c0rGbq+obBb4RNKBBGTdoWy+IHlObc5QOpEzubUmJ1VqmCTUyduKeOn24b+TvcmJ
i5PY1r8iKGhJJOAPt4KjBlIj67uqcGq3N9RA8pHQjn0ZXsfiLOmCeR6kFHbnNr4n
L7HvoEDR12K2Ci4+n7A/EAowHI/ZywcM7wADcWx4tOERPz0Pm2SUvVCjPVPc0xdN
KwIDAQAB
-----END PUBLIC KEY-----`

func dryrunJoin(t *testing.T, args ...string) dryruntypes.DryRun {
	if err := embedReleaseData(clusterConfigData); err != nil {
		t.Fatalf("fail to embed release data: %v", err)
	}

	if err := runInstallerCmd(
		append([]string{
			"join",
			"--yes",
		}, args...)...,
	); err != nil {
		t.Fatalf("fail to dryrun join embedded-cluster: %v", err)
	}

	dr, err := dryrun.Load()
	if err != nil {
		t.Fatalf("fail to unmarshal dryrun output: %v", err)
	}
	return *dr
}

func dryrunInstall(t *testing.T, c *dryrun.Client, args ...string) dryruntypes.DryRun {
	return dryrunInstallWithClusterConfig(t, c, clusterConfigData, args...)
}

func dryrunInstallWithClusterConfig(t *testing.T, c *dryrun.Client, clusterConfig string, args ...string) dryruntypes.DryRun {
	// Inject the dryrun test public key for license signature verification.
	// The kotskinds library uses a global custom key if set, otherwise looks up by key ID.
	// Our test license uses a test-only key that's not in kotskinds' default key map.
	if err := kotscrypto.SetCustomPublicKeyRSA(dryrunPublicKey); err != nil {
		t.Fatalf("failed to set custom public key: %v", err)
	}
	t.Cleanup(func() {
		kotscrypto.ResetCustomPublicKeyRSA()
	})

	if err := embedReleaseData(clusterConfig); err != nil {
		t.Fatalf("fail to embed release data: %v", err)
	}

	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	dryrun.Init(drFile, c)

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
	if err := embedReleaseData(clusterConfigData); err != nil {
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

func embedReleaseData(clusterConfig string) error {
	if err := release.SetReleaseDataForTests(map[string][]byte{
		"release.yaml":        []byte(releaseData),
		"cluster-config.yaml": []byte(clusterConfig),
		"application.yaml":    []byte(applicationData),
		"config.yaml":         []byte(configData),
		"chart.yaml":          []byte(helmChartData),
		"nginx-app-0.1.0.tgz": []byte(helmChartArchiveData),
	}); err != nil {
		return fmt.Errorf("set release data: %v", err)
	}
	return nil
}

func runInstallerCmd(args ...string) error {
	fullArgs := append([]string{"dryrun"}, args...)
	os.Args = fullArgs // for reporting

	installerCmd := cli.RootCmd(context.Background())
	installerCmd.SetArgs(args)
	return installerCmd.Execute()
}

func readK0sConfig(t *testing.T) k0sv1beta1.ClusterConfig {
	stdout, err := exec.Command("cat", runtimeconfig.K0sConfigPath).Output()
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
	t.Helper()

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
	t.Helper()

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
	t.Helper()

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
	t.Helper()

	for expectedKey, expectedValue := range expected {
		assert.Equal(t, expectedValue, actual[expectedKey])
	}
}

// assertCommands asserts that the expected commands are present in the actual commands
// if assertAll is true, it will fail the test if any command is present in the actual commands that was not expected
func assertCommands(t *testing.T, actual []dryruntypes.Command, expected []interface{}, assertAll bool) {
	t.Helper()

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

// findCommand finds the first command that matches the regex and returns it
// if no command is found, it returns nil
func findCommand(t *testing.T, commands []dryruntypes.Command, regex *regexp.Regexp) *dryruntypes.Command {
	for _, cmd := range commands {
		if regex.MatchString(cmd.Cmd) {
			return &cmd
		}
	}
	return nil
}

func assertConfigMapExists(t *testing.T, kcli client.Client, name string, namespace string) {
	t.Helper()

	var cm corev1.ConfigMap
	err := kcli.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &cm)
	assert.NoError(t, err, "failed to get configmap %s in namespace %s", name, namespace)
}

func assertSecretExists(t *testing.T, kcli client.Client, name string, namespace string) {
	t.Helper()

	var secret corev1.Secret
	err := kcli.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &secret)
	assert.NoError(t, err, "failed to get secret %s in namespace %s", name, namespace)
}

func assertSecretNotExists(t *testing.T, kcli client.Client, name string, namespace string) {
	t.Helper()

	var secret corev1.Secret
	err := kcli.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &secret)
	assert.Error(t, err, "secret %s should not exist in namespace %s", name, namespace)
}

func isHelmReleaseInstalled(hcli *helm.MockClient, releaseName string) (helm.InstallOptions, bool) {
	for _, call := range hcli.Calls {
		if call.Method == "Install" {
			opts := call.Arguments[1].(helm.InstallOptions)
			if opts.ReleaseName == releaseName {
				return opts, true
			}
		}
	}
	return helm.InstallOptions{}, false
}

func assertHelmValues(t *testing.T, actualValues map[string]interface{}, expectedValues map[string]interface{}) {
	t.Helper()

	for expectedKey, expectedValue := range expectedValues {
		actualValue, err := helm.GetValue(actualValues, expectedKey)
		assert.NoError(t, err)
		assert.Equal(t, expectedValue, actualValue, "expected value for key %s to be %v, got %v", expectedKey, expectedValue, actualValue)
	}
}

func assertHelmValuePrefixes(t *testing.T, actualValues map[string]interface{}, expectedPrefixes map[string]string) {
	t.Helper()

	for expectedKey, expectedPrefix := range expectedPrefixes {
		actualValue, err := helm.GetValue(actualValues, expectedKey)
		assert.NoError(t, err)
		if actualValue == nil {
			t.Errorf("expected prefix %s for key %s, got nil", expectedPrefix, expectedKey)
			return
		}

		actualValueStr, ok := actualValue.(string)
		if !ok {
			t.Errorf("expected prefix %s for key %s, got %v", expectedPrefix, expectedKey, actualValue)
			return
		}

		if !strings.HasPrefix(actualValueStr, expectedPrefix) {
			t.Errorf("expected prefix %s for key %s, got %s", expectedPrefix, expectedKey, actualValueStr)
			return
		}
	}
}

func getHelmExtraEnvValue(t *testing.T, values map[string]interface{}, key string, envName string) (string, bool) {
	extraEnvValue, err := helm.GetValue(values, key)
	require.NoError(t, err, "failed to get helm value for key %s", key)
	// this can be one of two types due to whether or not there are any overrides from the vendor
	// or end user as we call helm.PatchValues which marshals and unmarshals the values
	switch extraEnvValue := extraEnvValue.(type) {
	case []map[string]any:
		for _, env := range extraEnvValue {
			if env["name"] == envName {
				return env["value"].(string), true
			}
		}
	case []any:
		for _, env := range extraEnvValue {
			envMap, _ := env.(map[string]any)
			if envMap["name"] == envName {
				return envMap["value"].(string), true
			}
		}
	}
	return "", false
}

// createTarGzFile creates a valid tar.gz file with the given files and returns a ReadCloser
func createTarGzFile(t *testing.T, files map[string]string) io.ReadCloser {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Add each file to the tarball
	for fileName, content := range files {
		header := &tar.Header{
			Name: fileName,
			Size: int64(len(content)),
			Mode: 0644,
		}
		err := tw.WriteHeader(header)
		require.NoError(t, err)

		_, err = tw.Write([]byte(content))
		require.NoError(t, err)
	}

	// Close the tar writer and gzip writer
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	// Create a ReadCloser from the buffer
	return io.NopCloser(bytes.NewReader(buf.Bytes()))
}

func insecureHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy:           nil,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func waitForAPIReady(t *testing.T, hc *http.Client, url string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		resp, err := hc.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			return
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(250 * time.Millisecond)
	}
	require.FailNow(t, "API did not become ready in time")
}

func assertEventuallySucceeded(t *testing.T, contextMsg string, getStatus func() (apitypes.State, string, error)) {
	t.Helper()

	var lastState apitypes.State
	var lastMsg string
	var lastErr error

	timeout := 10 * time.Second
	interval := 100 * time.Millisecond

	ok := assert.Eventually(t, func() bool {
		st, msg, err := getStatus()
		lastState, lastMsg, lastErr = st, msg, err
		if err != nil {
			return false
		}
		return st == apitypes.StateSucceeded
	}, timeout, interval, "%s: lastState=%s, lastMsg=%s, lastErr=%v", contextMsg, lastState, lastMsg, lastErr)

	if !ok && lastState == apitypes.StateFailed {
		require.FailNowf(t, "operation failed", "%s: failed with message: %s", contextMsg, lastMsg)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0644), "failed to write file %s", path)
}
