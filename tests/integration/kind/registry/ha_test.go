package registry

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util/kind"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRegistry_EnableHAAirgap(t *testing.T) {
	ctx := t.Context()

	t.Log("building operator image")
	buildOperatorImage(t)

	util.SetupCtrlLogging(t)

	clusterName := util.GenerateClusterName(t)

	opts := &util.KindClusterOptions{
		NumControlPlaneNodes: 3,
	}
	kindConfig := util.NewKindClusterConfig(t, clusterName, opts)

	kindConfig.Nodes[0].ExtraPortMappings = append(kindConfig.Nodes[0].ExtraPortMappings, kind.PortMapping{
		ContainerPort: 30500,
	})

	kubeconfig := util.SetupKindClusterFromConfig(t, kindConfig)

	kcli := util.CtrlClient(t, kubeconfig)
	kclient := util.KubeClient(t, kubeconfig)
	hcli := util.HelmClient(t, kubeconfig)

	t.Log("installing openebs")
	addon := &openebs.OpenEBS{
		ProxyRegistryDomain: "proxy.replicated.com",
	}
	if err := addon.Install(ctx, kcli, hcli, nil, nil); err != nil {
		t.Fatalf("failed to install openebs: %v", err)
	}

	t.Log("waiting for storageclass")
	util.WaitForStorageClass(t, kubeconfig, "openebs-hostpath", 30*time.Second)

	t.Log("installing registry")
	registryAddon := &registry.Registry{
		ServiceCIDR:         "10.96.0.0/12",
		ProxyRegistryDomain: "proxy.replicated.com",
		IsHA:                false,
	}
	require.NoError(t, registryAddon.Install(ctx, kcli, hcli, nil, nil))

	t.Log("creating hostport service")
	registryAddr := createHostPortService(t, clusterName, kubeconfig)

	t.Log("installing admin console")
	adminConsoleAddon := &adminconsole.AdminConsole{
		IsAirgap:            true,
		ServiceCIDR:         "10.96.0.0/12",
		ProxyRegistryDomain: "proxy.replicated.com",
		IsHA:                false,
	}
	require.NoError(t, adminConsoleAddon.Install(ctx, kcli, hcli, nil, nil))

	t.Log("pushing image to registry")
	copyImageToRegistry(t, registryAddr, "docker.io/library/busybox:1.36.1")

	t.Log("running pod to validate image pull")
	runPodAndValidateImagePull(t, kubeconfig, "pod-1", "pod1.yaml")

	t.Log("creating installation with HA disabled")
	util.EnsureInstallation(t, kcli, ecv1beta1.InstallationSpec{
		HighAvailability: false,
	})

	cfgSpec := &ecv1beta1.ConfigSpec{
		Domains: ecv1beta1.Domains{
			ProxyRegistryDomain: "proxy.replicated.com",
		},
	}

	enableHAAndCancelContextOnMessage(t, kcli, kclient, hcli, cfgSpec,
		regexp.MustCompile(`StatefulSet is ready: seaweedfs`),
	)

	enableHAAndCancelContextOnMessage(t, kcli, kclient, hcli, cfgSpec,
		regexp.MustCompile(`Migrating data for high availability \(`),
	)

	enableHAAndCancelContextOnMessage(t, kcli, kclient, hcli, cfgSpec,
		regexp.MustCompile(`Updating the Admin Console for high availability`),
	)

	canEnable, reason, err := addons.CanEnableHA(t.Context(), kcli)
	require.NoError(t, err)
	require.True(t, canEnable, "should be able to enable HA: %s", reason)

	t.Log("enabling HA")
	loading := newTestingSpinner(t)
	func() {
		defer loading.Close()
		err = addons.EnableHA(ctx, kcli, kclient, hcli, true, "10.96.0.0/12", nil, cfgSpec, loading)
		require.NoError(t, err)
	}()

	t.Log("pushing a second image to registry")
	copyImageToRegistry(t, registryAddr, "docker.io/library/busybox:1.37.0")

	t.Log("running pod to validate image pull")
	runPodAndValidateImagePull(t, kubeconfig, "pod-1", "pod1.yaml")

	t.Log("running second pod to validate image pull")
	runPodAndValidateImagePull(t, kubeconfig, "pod-2", "pod2.yaml")
}

func enableHAAndCancelContextOnMessage(
	t *testing.T, kcli client.Client, kclient kubernetes.Interface, hcli helm.Client,
	cfgSpec *ecv1beta1.ConfigSpec,
	re *regexp.Regexp,
) {
	canEnable, reason, err := addons.CanEnableHA(t.Context(), kcli)
	require.NoError(t, err)
	require.True(t, canEnable, "should be able to enable HA: %s", reason)

	// we need to capture debug logs to trigger cancelation on matching messages
	logrus.SetLevel(logrus.DebugLevel)
	defer logrus.SetLevel(logrus.InfoLevel)
	logOut := logrus.StandardLogger().Out
	logrus.SetOutput(io.Discard)
	defer logrus.SetOutput(logOut)

	pr, pw := io.Pipe()
	defer pw.Close()

	// keep the original hooks to restore them later
	hooks := logrus.LevelHooks{}
	for _, levelHooks := range logrus.StandardLogger().Hooks {
		for _, hook := range levelHooks {
			hooks.Add(hook)
		}
	}
	defer logrus.StandardLogger().ReplaceHooks(hooks)

	// add the new hook to capture debug logs
	logrus.StandardLogger().AddHook(&logrusHook{writer: pw})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		defer pr.Close()
		got := waitForMatchingMessage(t, pr, re)
		if got {
			t.Log("cancelling context")
			cancel()
		}
		io.Copy(io.Discard, pr) // discard the rest of the output
	}()

	loading := newTestingSpinner(t)
	defer loading.Close()

	t.Log("enabling HA and cancelling context on message")
	err = addons.EnableHA(ctx, kcli, kclient, hcli, true, "10.96.0.0/12", nil, cfgSpec, loading)
	require.ErrorIs(t, err, context.Canceled, "expected context to be cancelled")
	t.Logf("cancelled context and got error: %v", err)
}

func waitForMatchingMessage(t *testing.T, r io.Reader, re *regexp.Regexp) bool {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		b := scanner.Bytes()
		// t.Logf("got message: %s", string(b))
		if re.Match(b) {
			t.Logf("got matching message: %s", string(b))
			return true
		}
	}

	return false
}

func buildOperatorImage(t *testing.T) string {
	// Get the directory of the current test file
	testDir := filepath.Dir(t.Name())
	// Go up three levels to reach the workspace root
	workspaceRoot := filepath.Join(testDir, "..", "..", "..")
	operatorDir := filepath.Join(workspaceRoot, "operator")

	if os.Getenv("SKIP_OPERATOR_IMAGE_BUILD") == "" {
		cmd := exec.CommandContext(t.Context(), "make", "-C", operatorDir,
			"build-and-push-operator-image", "USE_CHAINGUARD=0",
			"IMAGE_NAME=ttl.sh/replicated/embedded-cluster-operator-image",
		)

		var errBuf bytes.Buffer
		cmd.Stderr = &errBuf

		err := cmd.Run()
		if err != nil {
			t.Fatalf("failed to build operator image: %v: %s", err, errBuf.String())
		}
	}

	image, err := os.ReadFile(filepath.Join(operatorDir, "build/image"))
	if err != nil {
		t.Fatalf("failed to read operator image file: %v", err)
	}

	parts := strings.Split(strings.TrimSpace(string(image)), ":")
	if len(parts) != 2 {
		t.Fatalf("invalid operator image: %s", string(image))
	}

	embeddedclusteroperator.Metadata.Images["embedded-cluster-operator"] = release.AddonImage{
		Repo: parts[0],
		Tag: map[string]string{
			"amd64": parts[1],
			"arm64": parts[1],
		},
	}

	return string(image)
}

func newTestingSpinner(t *testing.T) *spinner.MessageWriter {
	return spinner.Start(
		spinner.WithWriter(func(format string, args ...any) (int, error) {
			// discard the output
			out := fmt.Sprintf(format, args...)
			t.Log("[spinner]", strings.TrimSpace(out))
			return len(out), nil
		}),
		spinner.WithTTY(false),
	)
}

type logrusHook struct {
	writer io.Writer
}

func (h *logrusHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.DebugLevel, logrus.InfoLevel}
}

func (h *logrusHook) Fire(entry *logrus.Entry) error {
	h.writer.Write([]byte(entry.Message + "\n"))
	return nil
}
