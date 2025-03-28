package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/lxd"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/stretchr/testify/require"
)

// SkipProxyTest returns true if the k0s version in use does not support
// proxied environments.
func SkipProxyTest() bool {
	supportedVersion := semver.MustParse("1.29.0")
	currentVersion := semver.MustParse(versions.K0sVersion)
	return currentVersion.LessThan(supportedVersion)
}

// TestProxiedEnvironment tests the installation behind a proxy server
func TestProxiedEnvironment(t *testing.T) {
	t.Parallel()
	if SkipProxyTest() {
		t.Skip("skipping test for k0s versions < 1.29.0")
	}

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                   t,
		Nodes:               4,
		WithProxy:           true,
		Image:               "debian/12",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()
	t.Log("Proxied infrastructure created")

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// install kots cli before configuring the proxy.
	t.Logf("%s: installing kots cli on node 0", time.Now().Format(time.RFC3339))
	line := []string{"install-kots-cli.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to install kots cli on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: reconfiguring squid to only allow whitelist access", time.Now().Format(time.RFC3339))
	line = []string{"enable-squid-whitelist.sh"}
	if _, _, err := tc.RunCommandOnProxyNode(t, line); err != nil {
		t.Fatalf("failed to reconfigure squid: %v", err)
	}

	t.Cleanup(func() {
		outputTCPDeniedLogs(t, tc)
	})

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
	installSingleNodeWithOptions(t, tc, installOptions{
		httpProxy:  lxd.HTTPProxy,
		httpsProxy: lxd.HTTPProxy,
		withEnv:    lxd.WithProxyEnv(tc.IPs),
	})

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// join a controller node
	joinControllerNode(t, tc, 1)

	// XXX If we are too aggressive joining nodes we can see the following error being
	// thrown by kotsadm on its log (and we get a 500 back):
	// "
	// failed to get controller role name: failed to get cluster config: failed to get
	// current installation: failed to list installations: etcdserver: leader changed
	// "
	t.Logf("node 1 joined, sleeping...")
	time.Sleep(30 * time.Second)

	// join another controller node
	joinControllerNode(t, tc, 2)

	// join a worker node
	joinWorkerNode(t, tc, 3)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 4, nil)

	// check the installation state
	checkInstallationState(t, tc)

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// TestProxiedCustomCIDR tests the installation behind a proxy server while using a custom pod and service CIDR
func TestProxiedCustomCIDR(t *testing.T) {
	t.Parallel()
	if SkipProxyTest() {
		t.Skip("skipping test for k0s versions < 1.29.0")
	}

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                   t,
		Nodes:               4,
		WithProxy:           true,
		Image:               "debian/12",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()
	t.Log("Proxied infrastructure created")

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// install kots cli before configuring the proxy.
	t.Logf("%s: installing kots cli on node 0", time.Now().Format(time.RFC3339))
	line := []string{"install-kots-cli.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to install kots cli on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: reconfiguring squid to only allow whitelist access", time.Now().Format(time.RFC3339))
	line = []string{"enable-squid-whitelist.sh"}
	if _, _, err := tc.RunCommandOnProxyNode(t, line); err != nil {
		t.Fatalf("failed to reconfigure squid: %v", err)
	}

	t.Cleanup(func() {
		outputTCPDeniedLogs(t, tc)
	})

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
	installSingleNodeWithOptions(t, tc, installOptions{
		httpProxy:   lxd.HTTPProxy,
		httpsProxy:  lxd.HTTPProxy,
		noProxy:     strings.Join(tc.IPs, ","),
		podCidr:     "10.128.0.0/20",
		serviceCidr: "10.129.0.0/20",
		withEnv:     lxd.WithProxyEnv(tc.IPs),
	})

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// join a controller node
	joinControllerNode(t, tc, 1)

	// XXX If we are too aggressive joining nodes we can see the following error being
	// thrown by kotsadm on its log (and we get a 500 back):
	// "
	// failed to get controller role name: failed to get cluster config: failed to get
	// current installation: failed to list installations: etcdserver: leader changed
	// "
	t.Logf("node 1 joined, sleeping...")
	time.Sleep(30 * time.Second)

	// join another controller node
	joinControllerNode(t, tc, 2)

	// join a worker node
	joinWorkerNode(t, tc, 3)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 4, nil)

	// check the installation state
	checkInstallationState(t, tc)

	// ensure that the cluster is using the right IP ranges.
	t.Logf("%s: checking service and pod IP addresses", time.Now().Format(time.RFC3339))
	stdout, _, err := tc.RunCommandOnNode(0, []string{"check-cidr-ranges.sh", "^10.128.[0-9]*.[0-9]", "^10.129.[0-9]*.[0-9]"})
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to check addresses on node %s: %v", tc.Nodes[0], err)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallWithMITMProxy(t *testing.T) {
	if SkipProxyTest() {
		t.Skip("skipping test for k0s versions < 1.29.0")
	}

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                   t,
		Nodes:               4,
		WithProxy:           true,
		Image:               "debian/12",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
		LicensePath:         "license.yaml",
	})
	defer tc.Cleanup()

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// install kots cli before configuring the proxy.
	t.Logf("%s: installing kots cli on node 0", time.Now().Format(time.RFC3339))
	line := []string{"install-kots-cli.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, lxd.WithMITMProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to install kots cli on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: reconfiguring squid to only allow whitelist access", time.Now().Format(time.RFC3339))
	line = []string{"enable-squid-whitelist.sh"}
	if _, _, err := tc.RunCommandOnProxyNode(t, line); err != nil {
		t.Fatalf("failed to reconfigure squid: %v", err)
	}

	t.Cleanup(func() {
		outputTCPDeniedLogs(t, tc)
	})

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
	installSingleNodeWithOptions(t, tc, installOptions{
		httpProxy:  lxd.HTTPMITMProxy,
		httpsProxy: lxd.HTTPMITMProxy,
		privateCA:  "/usr/local/share/ca-certificates/proxy/ca.crt",
		withEnv:    lxd.WithMITMProxyEnv(tc.IPs),
	})

	_, _, err := tc.SetupPlaywrightAndRunTest("deploy-app")
	require.NoError(t, err, "failed to deploy app")

	// join a controller node
	joinControllerNode(t, tc, 1)

	// XXX If we are too aggressive joining nodes we can see the following error being
	// thrown by kotsadm on its log (and we get a 500 back):
	// "
	// failed to get controller role name: failed to get cluster config: failed to get
	// current installation: failed to list installations: etcdserver: leader changed
	// "
	t.Logf("node 1 joined, sleeping...")
	time.Sleep(30 * time.Second)

	// join another controller node
	joinControllerNode(t, tc, 2)

	// join a worker node
	joinWorkerNode(t, tc, 3)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 4, nil)

	// check the installation state
	checkInstallationState(t, tc)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func outputTCPDeniedLogs(t *testing.T, tc *lxd.Cluster) {
	stdout, _, err := tc.RunCommandOnProxyNode(t, []string{"sh", "-c", `grep -A1 TCP_DENIED /var/log/squid/access.log | grep -v speedtest\.net`})
	if err != nil {
		t.Fatalf("fail to check squid access log: %v", err)
	}
	t.Logf("TCP_DENIED logs:")
	t.Log(stdout)
	if strings.Contains(stdout, "TCP_DENIED") {
		t.Fatalf("TCP_DENIED logs found")
	}
}
