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
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
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
	// Each node needs its own separate data directory to avoid conflicts with local persistent volumes
	for i := range kindConfig.Nodes {
		ecDataDirMount := kind.Mount{
			HostPath:      util.TempDirForHostMount(t, "data-dir-*"),
			ContainerPath: "/var/lib/embedded-cluster",
		}
		k0sDirMount := kind.Mount{
			HostPath:      util.TempDirForHostMount(t, "k0s-dir-*"),
			ContainerPath: "/var/lib/embedded-cluster/k0s",
		}
		kindConfig.Nodes[i].ExtraMounts = append(kindConfig.Nodes[i].ExtraMounts, ecDataDirMount, k0sDirMount)
	}

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

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, kcli)
	require.NoError(t, err)

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
		KotsadmNamespace:   kotsadmNamespace,
	}
	require.NoError(t, adminConsoleAddon.Install(ctx, t.Logf, kcli, mcli, hcli, domains, nil))

	t.Logf("%s pushing image to registry", formattedTime())
	copyImageToRegistry(t, registryAddr, "docker.io/library/busybox:1.36.0")

	t.Logf("%s running pod to validate image pull", formattedTime())
	runPodAndValidateImagePull(t, kubeconfig, kotsadmNamespace, "pod-1", "pod1.yaml")

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

	enableHAAndCancelContextOnMessage(t, kubeconfig, addOns, inSpec,
		regexp.MustCompile(`StatefulSet is ready: seaweedfs`),
	)

	enableHAAndCancelContextOnMessage(t, kubeconfig, addOns, inSpec,
		regexp.MustCompile(`Migrating data for high availability \(`),
	)

	enableHAAndCancelContextOnMessage(t, kubeconfig, addOns, inSpec,
		regexp.MustCompile(`Updating the Admin Console for high availability`),
	)

	canEnable, reason, err := addOns.CanEnableHA(t.Context())
	require.NoError(t, err)
	require.True(t, canEnable, "should be able to enable HA: %s", reason)

	t.Logf("%s enabling HA", formattedTime())
	err = enableHA(ctx, t, addOns, inSpec)
	require.NoError(t, err, "failed to enable HA")

	t.Logf("%s validating seaweedfs helm values", formattedTime())
	describeOut := util.K8sDescribe(t, kubeconfig, "seaweedfs", "statefulset", "seaweedfs-master")
	if !strings.Contains(describeOut, "raftHashicorp") {
		t.Fatalf("seaweedfs helm values should contain raftHashicorp")
	}
	if !strings.Contains(describeOut, "raftBootstrap") {
		t.Fatalf("seaweedfs helm values should contain raftBootstrap")
	}

	t.Logf("%s pushing a second image to registry", formattedTime())
	copyImageToRegistry(t, registryAddr, "docker.io/library/busybox:1.36.1")

	t.Logf("%s running pod to validate image pull", formattedTime())
	runPodAndValidateImagePull(t, kubeconfig, kotsadmNamespace, "pod-1", "pod1.yaml")

	t.Logf("%s running second pod to validate image pull", formattedTime())
	runPodAndValidateImagePull(t, kubeconfig, kotsadmNamespace, "pod-2", "pod2.yaml")
}

// If this function name is changed, the .github/workflows/ci.yaml file needs to be updated
// to match the new function name.
func TestRegistry_DisableHashiRaft(t *testing.T) {
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
	// Each node needs its own separate data directory to avoid conflicts with local persistent volumes
	for i := range kindConfig.Nodes {
		ecDataDirMount := kind.Mount{
			HostPath:      util.TempDirForHostMount(t, "data-dir-*"),
			ContainerPath: "/var/lib/embedded-cluster",
		}
		k0sDirMount := kind.Mount{
			HostPath:      util.TempDirForHostMount(t, "k0s-dir-*"),
			ContainerPath: "/var/lib/embedded-cluster/k0s",
		}
		kindConfig.Nodes[i].ExtraMounts = append(kindConfig.Nodes[i].ExtraMounts, ecDataDirMount, k0sDirMount)
	}

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

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, kcli)
	require.NoError(t, err)

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
		KotsadmNamespace:   kotsadmNamespace,
	}
	require.NoError(t, adminConsoleAddon.Install(ctx, t.Logf, kcli, mcli, hcli, domains, nil))

	t.Logf("%s pushing image to registry", formattedTime())
	copyImageToRegistry(t, registryAddr, "docker.io/library/busybox:1.36.0")

	t.Logf("%s running pod to validate image pull", formattedTime())
	runPodAndValidateImagePull(t, kubeconfig, kotsadmNamespace, "pod-1", "pod1.yaml")

	t.Logf("%s creating installation with HA disabled", formattedTime())
	util.EnsureInstallation(t, kcli, ecv1beta1.InstallationSpec{
		HighAvailability: false,
	})

	inSpec := ecv1beta1.InstallationSpec{
		AirGap: true,
		Config: &ecv1beta1.ConfigSpec{
			Domains: domains,
			UnsupportedOverrides: ecv1beta1.UnsupportedOverrides{
				BuiltInExtensions: []ecv1beta1.BuiltInExtension{
					{
						Name:   "seaweedfs",
						Values: "master:\n  raftHashicorp: false\n  raftBootstrap: false\n",
					},
				},
			},
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

	enableHAAndCancelContextOnMessage(t, kubeconfig, addOns, inSpec,
		regexp.MustCompile(`StatefulSet is ready: seaweedfs`),
	)

	enableHAAndCancelContextOnMessage(t, kubeconfig, addOns, inSpec,
		regexp.MustCompile(`Migrating data for high availability \(`),
	)

	enableHAAndCancelContextOnMessage(t, kubeconfig, addOns, inSpec,
		regexp.MustCompile(`Updating the Admin Console for high availability`),
	)

	canEnable, reason, err := addOns.CanEnableHA(t.Context())
	require.NoError(t, err)
	require.True(t, canEnable, "should be able to enable HA: %s", reason)

	t.Logf("%s enabling HA", formattedTime())
	err = enableHA(ctx, t, addOns, inSpec)
	require.NoError(t, err, "failed to enable HA")

	t.Logf("%s validating seaweedfs helm values", formattedTime())
	describeOut := util.K8sDescribe(t, kubeconfig, "seaweedfs", "statefulset", "seaweedfs-master")
	if strings.Contains(describeOut, "raftHashicorp") {
		t.Fatalf("seaweedfs helm values should not contain raftHashicorp")
	}
	if strings.Contains(describeOut, "raftBootstrap") {
		t.Fatalf("seaweedfs helm values should not contain raftBootstrap")
	}

	t.Logf("%s pushing a second image to registry", formattedTime())
	copyImageToRegistry(t, registryAddr, "docker.io/library/busybox:1.36.1")

	t.Logf("%s running pod to validate image pull", formattedTime())
	runPodAndValidateImagePull(t, kubeconfig, kotsadmNamespace, "pod-1", "pod1.yaml")

	t.Logf("%s running second pod to validate image pull", formattedTime())
	runPodAndValidateImagePull(t, kubeconfig, kotsadmNamespace, "pod-2", "pod2.yaml")

	t.Logf("%s upgrading seaweedfs, raftHashicorp and raftBootstrap should stay disabled", formattedTime())
	seaweedfsAddon := &seaweedfs.SeaweedFS{
		ServiceCIDR:      rc.ServiceCIDR(),
		SeaweedFSDataDir: rc.EmbeddedClusterSeaweedFSSubDir(),
	}
	require.NoError(t, seaweedfsAddon.Upgrade(t.Context(), t.Logf, kcli, mcli, hcli, domains, nil))

	t.Logf("%s validating seaweedfs helm values", formattedTime())
	describeOut = util.K8sDescribe(t, kubeconfig, "seaweedfs", "statefulset", "seaweedfs-master")
	if strings.Contains(describeOut, "raftHashicorp") {
		t.Fatalf("seaweedfs helm values should not contain raftHashicorp")
	}
	if strings.Contains(describeOut, "raftBootstrap") {
		t.Fatalf("seaweedfs helm values should not contain raftBootstrap")
	}

	t.Logf("%s pushing a third image to registry", formattedTime())
	copyImageToRegistry(t, registryAddr, "docker.io/library/busybox:1.37.0")

	t.Logf("%s running pod to validate image pull", formattedTime())
	runPodAndValidateImagePull(t, kubeconfig, kotsadmNamespace, "pod-1", "pod1.yaml")

	t.Logf("%s running second pod to validate image pull", formattedTime())
	runPodAndValidateImagePull(t, kubeconfig, kotsadmNamespace, "pod-2", "pod2.yaml")

	t.Logf("%s running third pod to validate image pull", formattedTime())
	runPodAndValidateImagePull(t, kubeconfig, kotsadmNamespace, "pod-3", "pod3.yaml")
}

func enableHAAndCancelContextOnMessage(t *testing.T, kubeconfig string, addOns *addons.AddOns, inSpec ecv1beta1.InstallationSpec, re *regexp.Regexp) {
	t.Cleanup(func() {
		if t.Failed() {
			printSeaweedFSDebugInfo(t, kubeconfig)
		}
	})

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

	t.Logf("%s enabling HA and cancelling context on message", formattedTime())
	err = enableHA(ctx, t, addOns, inSpec)
	require.ErrorIs(t, err, context.Canceled, "expected context to be cancelled")
	t.Logf("%s cancelled context and got error: %v", formattedTime(), err)
}

func enableHA(ctx context.Context, t *testing.T, addOns *addons.AddOns, inSpec ecv1beta1.InstallationSpec) error {
	loading := newTestingSpinner(t)
	defer loading.Close()

	rc := runtimeconfig.New(inSpec.RuntimeConfig)

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, nil)
	require.NoError(t, err)

	opts := addons.EnableHAOptions{
		ClusterID:          "123",
		AdminConsolePort:   rc.AdminConsolePort(),
		IsAirgap:           true,
		IsMultiNodeEnabled: false,
		EmbeddedConfigSpec: inSpec.Config,
		EndUserConfigSpec:  nil,
		ProxySpec:          rc.ProxySpec(),
		HostCABundlePath:   rc.HostCABundlePath(),
		DataDir:            rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:         rc.EmbeddedClusterK0sSubDir(),
		SeaweedFSDataDir:   rc.EmbeddedClusterSeaweedFSSubDir(),
		ServiceCIDR:        inSpec.RuntimeConfig.Network.ServiceCIDR,
		KotsadmNamespace:   kotsadmNamespace,
	}
	return addOns.EnableHA(ctx, opts, loading)
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

func printSeaweedFSDebugInfo(t *testing.T, kubeconfig string) {
	t.Logf("%s ===== SEAWEEDFS DEBUG INFO =====", formattedTime())

	// Get StatefulSet status
	t.Logf("%s --- SeaweedFS Master StatefulSet Status ---", formattedTime())
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfig, "get", "statefulset", "seaweedfs-master", "-n", "seaweedfs", "-o", "wide")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Logf("%s\n%s", formattedTime(), string(out))
	} else {
		t.Logf("%s Failed to get statefulset: %v\n%s", formattedTime(), err, string(out))
	}

	// Describe StatefulSet
	t.Logf("%s --- SeaweedFS Master StatefulSet Describe ---", formattedTime())
	cmd = exec.Command("kubectl", "--kubeconfig", kubeconfig, "describe", "statefulset", "seaweedfs-master", "-n", "seaweedfs")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Logf("%s\n%s", formattedTime(), string(out))
	} else {
		t.Logf("%s Failed to describe statefulset: %v\n%s", formattedTime(), err, string(out))
	}

	// Get Pods status
	t.Logf("%s --- SeaweedFS Pods ---", formattedTime())
	cmd = exec.Command("kubectl", "--kubeconfig", kubeconfig, "get", "pods", "-n", "seaweedfs", "-o", "wide")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Logf("%s\n%s", formattedTime(), string(out))
	} else {
		t.Logf("%s Failed to get pods: %v\n%s", formattedTime(), err, string(out))
	}

	// Describe each pod
	cmd = exec.Command("kubectl", "--kubeconfig", kubeconfig, "get", "pods", "-n", "seaweedfs", "-o", "jsonpath={.items[*].metadata.name}")
	if out, err := cmd.CombinedOutput(); err == nil {
		podNames := strings.Fields(string(out))
		for _, podName := range podNames {
			t.Logf("%s --- Pod %s Describe ---", formattedTime(), podName)
			descCmd := exec.Command("kubectl", "--kubeconfig", kubeconfig, "describe", "pod", podName, "-n", "seaweedfs")
			if descOut, descErr := descCmd.CombinedOutput(); descErr == nil {
				t.Logf("%s\n%s", formattedTime(), string(descOut))
			} else {
				t.Logf("%s Failed to describe pod %s: %v", formattedTime(), podName, descErr)
			}

			// Get pod logs
			t.Logf("%s --- Pod %s Logs ---", formattedTime(), podName)
			logCmd := exec.Command("kubectl", "--kubeconfig", kubeconfig, "logs", podName, "-n", "seaweedfs", "--tail=100")
			if logOut, logErr := logCmd.CombinedOutput(); logErr == nil {
				t.Logf("%s\n%s", formattedTime(), string(logOut))
			} else {
				t.Logf("%s Failed to get logs for pod %s: %v\n%s", formattedTime(), podName, logErr, string(logOut))
			}
		}
	}

	// Get events
	t.Logf("%s --- SeaweedFS Namespace Events ---", formattedTime())
	cmd = exec.Command("kubectl", "--kubeconfig", kubeconfig, "get", "events", "-n", "seaweedfs", "--sort-by=.lastTimestamp")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Logf("%s\n%s", formattedTime(), string(out))
	} else {
		t.Logf("%s Failed to get events: %v\n%s", formattedTime(), err, string(out))
	}

	// Get PVCs
	t.Logf("%s --- SeaweedFS PVCs ---", formattedTime())
	cmd = exec.Command("kubectl", "--kubeconfig", kubeconfig, "get", "pvc", "-n", "seaweedfs", "-o", "wide")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Logf("%s\n%s", formattedTime(), string(out))
	} else {
		t.Logf("%s Failed to get PVCs: %v\n%s", formattedTime(), err, string(out))
	}

	t.Logf("%s ===== END SEAWEEDFS DEBUG INFO =====", formattedTime())
}

func formattedTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
