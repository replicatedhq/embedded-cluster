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
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util/kind"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// If this function name is changed, the .github/workflows/ci.yaml file needs to be updated
// to match the new function name.
func TestRegistry_EnableHAAirgap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	ctx := t.Context()

	buildOperatorImage(t)

	util.SetupCtrlLogging(t)

	clusterName := util.GenerateClusterName(t)

	kindConfig := util.NewKindClusterConfig(t, clusterName, &util.KindClusterOptions{
		NumControlPlaneNodes: 3,
	})

	kindConfig.Nodes[0].ExtraPortMappings = append(kindConfig.Nodes[0].ExtraPortMappings, kind.PortMapping{
		ContainerPort: 30500,
	})

	// data and k0s directories are required for the admin console addon
	ecDataDirMount := kind.Mount{
		HostPath:      util.TempDirForHostMount(t, "data-dir-*"),
		ContainerPath: "/var/lib/embedded-cluster",
	}
	k0sDirMount := kind.Mount{
		HostPath:      util.TempDirForHostMount(t, "k0s-dir-*"),
		ContainerPath: "/var/lib/embedded-cluster/k0s",
	}
	kindConfig.Nodes[0].ExtraMounts = append(kindConfig.Nodes[0].ExtraMounts, ecDataDirMount, k0sDirMount)
	kindConfig.Nodes[1].ExtraMounts = append(kindConfig.Nodes[1].ExtraMounts, ecDataDirMount, k0sDirMount)
	kindConfig.Nodes[2].ExtraMounts = append(kindConfig.Nodes[2].ExtraMounts, ecDataDirMount, k0sDirMount)

	kubeconfig := util.SetupKindClusterFromConfig(t, kindConfig)

	kcli := util.CtrlClient(t, kubeconfig)
	mcli := util.MetadataClient(t, kubeconfig)
	kclient := util.KubeClient(t, kubeconfig)
	hcli := util.HelmClient(t, kubeconfig)

	rc := runtimeconfig.New(nil)
	rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
		PodCIDR:     "10.85.0.0/12",
		ServiceCIDR: "10.96.0.0/12",
	})

	domains := ecv1beta1.Domains{
		ReplicatedAppDomain:      "replicated.app",
		ProxyRegistryDomain:      "proxy.replicated.com",
		ReplicatedRegistryDomain: "registry.replicated.com",
	}

	t.Logf("%s installing openebs", formattedTime())
	addon := &openebs.OpenEBS{
		OpenEBSDataDir: rc.EmbeddedClusterOpenEBSLocalSubDir(),
	}
	if err := addon.Install(ctx, t.Logf, kcli, mcli, hcli, domains, nil); err != nil {
		t.Fatalf("failed to install openebs: %v", err)
	}

	t.Logf("%s waiting for storageclass", formattedTime())
	util.WaitForStorageClass(t, kubeconfig, "openebs-hostpath", 30*time.Second)

	t.Logf("%s installing registry", formattedTime())
	registryAddon := &registry.Registry{
		ServiceCIDR: "10.96.0.0/12",
		IsHA:        false,
	}
	require.NoError(t, registryAddon.Install(ctx, t.Logf, kcli, mcli, hcli, domains, nil))

	t.Logf("%s creating hostport service", formattedTime())
	registryAddr := createHostPortService(t, clusterName, kubeconfig)

	t.Logf("%s installing admin console", formattedTime())
	adminConsoleAddon := &adminconsole.AdminConsole{
		ClusterID:          "123",
		IsAirgap:           true,
		IsHA:               false,
		Proxy:              rc.ProxySpec(),
		ServiceCIDR:        "10.96.0.0/12",
		IsMultiNodeEnabled: false,
		HostCABundlePath:   rc.HostCABundlePath(),
		DataDir:            rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:         rc.EmbeddedClusterK0sSubDir(),
		AdminConsolePort:   rc.AdminConsolePort(),
	}
	require.NoError(t, adminConsoleAddon.Install(ctx, t.Logf, kcli, mcli, hcli, domains, nil))

	t.Logf("%s pushing image to registry", formattedTime())
	copyImageToRegistry(t, registryAddr, "docker.io/library/busybox:1.36.1")

	t.Logf("%s running pod to validate image pull", formattedTime())
	runPodAndValidateImagePull(t, kubeconfig, "pod-1", "pod1.yaml")

	t.Logf("%s creating installation with HA disabled", formattedTime())
	util.EnsureInstallation(t, kcli, ecv1beta1.InstallationSpec{
		HighAvailability: false,
	})

	inSpec := ecv1beta1.InstallationSpec{
		AirGap: true,
		Config: &ecv1beta1.ConfigSpec{
			Domains: domains,
		},
		RuntimeConfig: rc.Get(),
	}

	addOns := addons.New(
		addons.WithKubernetesClient(kcli),
		addons.WithKubernetesClientSet(kclient),
		addons.WithMetadataClient(mcli),
		addons.WithHelmClient(hcli),
		addons.WithDomains(domains),
	)

	enableHAAndCancelContextOnMessage(t, addOns, inSpec,
		regexp.MustCompile(`StatefulSet is ready: seaweedfs`),
	)

	enableHAAndCancelContextOnMessage(t, addOns, inSpec,
		regexp.MustCompile(`Migrating data for high availability \(`),
	)

	enableHAAndCancelContextOnMessage(t, addOns, inSpec,
		regexp.MustCompile(`Updating the Admin Console for high availability`),
	)

	canEnable, reason, err := addOns.CanEnableHA(t.Context())
	require.NoError(t, err)
	require.True(t, canEnable, "should be able to enable HA: %s", reason)

	t.Logf("%s enabling HA", formattedTime())
	loading := newTestingSpinner(t)
	func() {
		defer loading.Close()
		opts := addons.EnableHAOptions{
			ClusterID:   "123",
			ServiceCIDR: rc.ServiceCIDR(),
			ProxySpec:   rc.ProxySpec(),
		}
		err = addOns.EnableHA(t.Context(), opts, loading)
		require.NoError(t, err)
	}()

	t.Logf("%s pushing a second image to registry", formattedTime())
	copyImageToRegistry(t, registryAddr, "docker.io/library/busybox:1.37.0")

	t.Logf("%s running pod to validate image pull", formattedTime())
	runPodAndValidateImagePull(t, kubeconfig, "pod-1", "pod1.yaml")

	t.Logf("%s running second pod to validate image pull", formattedTime())
	runPodAndValidateImagePull(t, kubeconfig, "pod-2", "pod2.yaml")
}

func enableHAAndCancelContextOnMessage(t *testing.T, addOns *addons.AddOns, inSpec ecv1beta1.InstallationSpec, re *regexp.Regexp) {
	canEnable, reason, err := addOns.CanEnableHA(t.Context())
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
			t.Logf("%s cancelling context", formattedTime())
			cancel()
		}
		io.Copy(io.Discard, pr) // discard the rest of the output
	}()

	loading := newTestingSpinner(t)
	defer loading.Close()

	rc := runtimeconfig.New(inSpec.RuntimeConfig)

	t.Logf("%s enabling HA and cancelling context on message", formattedTime())
	opts := addons.EnableHAOptions{
		ClusterID:          "123",
		AdminConsolePort:   rc.AdminConsolePort(),
		IsAirgap:           true,
		IsMultiNodeEnabled: false,
		EmbeddedConfigSpec: inSpec.Config,
		EndUserConfigSpec:  inSpec.Config,
		ProxySpec:          rc.ProxySpec(),
		HostCABundlePath:   rc.HostCABundlePath(),
		DataDir:            rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:         rc.EmbeddedClusterK0sSubDir(),
		SeaweedFSDataDir:   rc.EmbeddedClusterSeaweedFSSubDir(),
		ServiceCIDR:        inSpec.RuntimeConfig.Network.ServiceCIDR,
	}
	err = addOns.EnableHA(ctx, opts, loading)
	require.ErrorIs(t, err, context.Canceled, "expected context to be cancelled")
	t.Logf("%s cancelled context and got error: %v", formattedTime(), err)
}

func waitForMatchingMessage(t *testing.T, r io.Reader, re *regexp.Regexp) bool {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		b := scanner.Bytes()
		if re.Match(b) {
			t.Logf("%s got matching message: %s", formattedTime(), string(b))
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
		t.Logf("%s building operator image", formattedTime())

		cmd := exec.CommandContext(
			t.Context(), "make", "-C", operatorDir, "build-ttl.sh", "USE_CHAINGUARD=0",
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
			t.Logf("%s [spinner] %s", formattedTime(), strings.TrimSpace(out))
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

func formattedTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
