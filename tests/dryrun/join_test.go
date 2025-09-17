package dryrun

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types/join"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nodeutil "k8s.io/component-helpers/node/util"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestJoinControllerNode(t *testing.T) {
	testJoinControllerNodeImpl(t, false, false)
}

func TestJoinAirgapControllerNode(t *testing.T) {
	testJoinControllerNodeImpl(t, true, false)
}

func TestJoinHAMigrationControllerNode(t *testing.T) {
	testJoinControllerNodeImpl(t, false, true)
}

func TestJoinHAMigrationAirgapControllerNode(t *testing.T) {
	testJoinControllerNodeImpl(t, true, true)
}

func testJoinControllerNodeImpl(t *testing.T, isAirgap bool, hasHAMigration bool) {
	clusterID := uuid.New()
	jcmd := &join.JoinCommandResponse{
		K0sJoinCommand:         "/usr/local/bin/k0s install controller --enable-worker --no-taints --labels kots.io/embedded-cluster-role=total-1,kots.io/embedded-cluster-role-0=controller-test,controller-label=controller-label-value",
		K0sToken:               "some-k0s-token",
		EmbeddedClusterVersion: "v0.0.0",
		ClusterID:              clusterID,
		InstallationSpec: ecv1beta1.InstallationSpec{
			ClusterID:      clusterID.String(),
			MetricsBaseURL: "https://testing.com",
			Config:         &ecv1beta1.ConfigSpec{UnsupportedOverrides: ecv1beta1.UnsupportedOverrides{}},
			AirGap:         isAirgap,
			RuntimeConfig: &ecv1beta1.RuntimeConfigSpec{
				Network: ecv1beta1.NetworkSpec{
					NetworkInterface: "ens1",
					PodCIDR:          "10.2.0.0/17",
					ServiceCIDR:      "10.2.128.0/17",
				},
			},
		},
		TCPConnectionsRequired: []string{"10.0.0.1:6443", "10.0.0.1:9443"},
	}

	kotsadm := dryrun.NewKotsadm()
	kubeUtils := &dryrun.KubeUtils{}

	hcli := &helm.MockClient{}
	hcli.On("Close").Once().Return(nil)

	// Validate join against from a node with a different network interface from the first node
	ip := net.ParseIP("192.168.1.60")
	ifaceProvider := &dryrun.NetworkInterfaceProvider{
		Ifaces: []netutils.NetworkInterface{
			&dryrun.NetworkInterface{
				MockName:  "eth0",
				MockFlags: net.FlagUp,
				MockAddrs: []net.Addr{
					&net.IPNet{IP: ip, Mask: net.CIDRMask(24, 32)},
				},
			},
		},
	}
	chooseIface := &dryrun.ChooseInterfaceImpl{IP: ip}

	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	dryrun.Init(drFile, &dryrun.Client{
		Kotsadm:                  kotsadm,
		KubeUtils:                kubeUtils,
		HelmClient:               hcli,
		NetworkInterfaceProvider: ifaceProvider,
		ChooseHostInterfaceImpl:  chooseIface,
	})

	kotsadm.SetGetJoinTokenResponse("10.0.0.1", "some-token", jcmd, nil)

	if isAirgap {
		// make sure k0s images file does not exist before join
		_, err := os.ReadFile("/var/lib/embedded-cluster/k0s/images/ec-images-amd64.tar")
		require.ErrorIs(t, err, os.ErrNotExist)

		// make sure charts directory does not exist before join
		_, err = os.ReadFile("/var/lib/embedded-cluster/charts")
		require.ErrorIs(t, err, os.ErrNotExist)

		// create fake k0s images file
		testK0sImagesPath := filepath.Join(t.TempDir(), "ec-images-amd64.tar")
		err = os.WriteFile(testK0sImagesPath, []byte("fake-k0s-images-file-content"), 0644)
		require.NoError(t, err)

		testK0sImagesFile, err := os.Open(testK0sImagesPath)
		require.NoError(t, err)
		defer testK0sImagesFile.Close()

		kotsadm.SetGetK0sImagesFileResponse("10.0.0.1", testK0sImagesFile, nil)

		// create fake charts tar.gz file
		chartFiles := map[string]string{
			"seaweedfs-4.0.379.tgz":           "fake-seaweedfs-chart-content",
			"docker-registry-2.2.3.tgz":       "fake-docker-registry-chart-content",
			"admin-console-1.124.15-ec.1.tgz": "fake-admin-console-chart-content",
		}
		testChartsFile := createTarGzFile(t, chartFiles)
		defer testChartsFile.Close()

		kotsadm.SetGetECChartsResponse("10.0.0.1", testChartsFile, nil)
	}

	kcli, err := kubeUtils.KubeClient()
	require.NoError(t, err)

	kcli.Create(context.Background(), &ecv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Installation",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "20241002205018",
		},
		Spec: ecv1beta1.InstallationSpec{
			ClusterID:        clusterID.String(),
			HighAvailability: false,
			Config: &ecv1beta1.ConfigSpec{
				Version: "2.2.0+k8s-1.30",
			},
			RuntimeConfig: &ecv1beta1.RuntimeConfigSpec{},
		},
	}, &ctrlclient.CreateOptions{})

	kcli.Create(context.Background(), &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "true",
			},
		},
	}, &ctrlclient.CreateOptions{})
	kcli.Create(context.Background(), &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-2",
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "true",
			},
		},
	}, &ctrlclient.CreateOptions{})

	if hasHAMigration {
		kcli.Create(context.Background(), &corev1.Node{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Node",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-3",
				Labels: map[string]string{
					"node-role.kubernetes.io/control-plane": "true",
				},
			},
		}, &ctrlclient.CreateOptions{})
		kcli.Create(context.Background(), &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "registry",
				Namespace: "registry",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
			},
		}, &ctrlclient.CreateOptions{})

		if isAirgap {
			hcli.On("ReleaseExists", mock.Anything, "seaweedfs", "seaweedfs").Once().Return(true, nil)
			hcli.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
				return opts.ReleaseName == "seaweedfs"
			})).Once().Return(nil, nil)
			hcli.On("ReleaseExists", mock.Anything, "registry", "docker-registry").Once().Return(true, nil)
			hcli.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
				return opts.ReleaseName == "docker-registry"
			})).Once().Return(nil, nil)
		}
		hcli.On("ReleaseExists", mock.Anything, "kotsadm", "admin-console").Once().Return(true, nil)
		hcli.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "admin-console"
		})).Once().Return(nil, nil)
	}

	dr := dryrunJoin(t, "10.0.0.1", "some-token")

	// --- validate k0s images file and charts (if airgap) --- //
	if isAirgap {
		// validate that k0s images were written
		content, err := os.ReadFile("/var/lib/embedded-cluster/k0s/images/ec-images-amd64.tar")
		require.NoError(t, err)
		assert.Equal(t, "fake-k0s-images-file-content", string(content))

		// validate that charts were extracted and written to the correct directory
		chartsDir := "/var/lib/embedded-cluster/charts"
		content, err = os.ReadFile(filepath.Join(chartsDir, "seaweedfs-4.0.379.tgz"))
		require.NoError(t, err)
		assert.Equal(t, "fake-seaweedfs-chart-content", string(content))

		content, err = os.ReadFile(filepath.Join(chartsDir, "docker-registry-2.2.3.tgz"))
		require.NoError(t, err)
		assert.Equal(t, "fake-docker-registry-chart-content", string(content))

		content, err = os.ReadFile(filepath.Join(chartsDir, "admin-console-1.124.15-ec.1.tgz"))
		require.NoError(t, err)
		assert.Equal(t, "fake-admin-console-chart-content", string(content))
	}

	// --- validate host preflight spec --- //
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"TCPConnect-0": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.TCPConnect != nil && strings.HasPrefix(hc.TCPConnect.CollectorName, "tcp-connect-0")
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, "10.0.0.1:6443", hc.TCPConnect.Address)
			},
		},
		"TCPConnect-1": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.TCPConnect != nil && strings.HasPrefix(hc.TCPConnect.CollectorName, "tcp-connect-1")
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, "10.0.0.1:9443", hc.TCPConnect.Address)
			},
		},
	})

	assertAnalyzers(t, dr.HostPreflightSpec.Analyzers, map[string]struct {
		match    func(*troubleshootv1beta2.HostAnalyze) bool
		validate func(*troubleshootv1beta2.HostAnalyze)
	}{
		"TCPConnect-0": {
			match: func(hc *troubleshootv1beta2.HostAnalyze) bool {
				return hc.TCPConnect != nil && strings.HasPrefix(hc.TCPConnect.CollectorName, "tcp-connect-0")
			},
			validate: func(hc *troubleshootv1beta2.HostAnalyze) {
				assert.Contains(t, hc.TCPConnect.Outcomes, &troubleshootv1beta2.Outcome{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "connection-refused",
						Message: "A TCP connection to 10.0.0.1:6443 is required, but the connection was refused. This can occur, for example, if IP routing is not possible between this host and 10.0.0.1:6443, or if your firewall doesn't allow traffic between this host and 10.0.0.1:6443.",
					},
				})
				assert.Contains(t, hc.TCPConnect.Outcomes, &troubleshootv1beta2.Outcome{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "connection-timeout",
						Message: "A TCP connection to 10.0.0.1:6443 is required, but the connection timed out. This can occur, for example, if IP routing is not possible between this host and 10.0.0.1:6443, or if your firewall doesn't allow traffic between this host and 10.0.0.1:6443.",
					},
				})
				assert.Contains(t, hc.TCPConnect.Outcomes, &troubleshootv1beta2.Outcome{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "error",
						Message: "A TCP connection to 10.0.0.1:6443 is required, but an unexpected error occurred. This can occur, for example, if IP routing is not possible between this host and 10.0.0.1:6443, or if your firewall doesn't allow traffic between this host and 10.0.0.1:6443.",
					},
				})
				assert.Contains(t, hc.TCPConnect.Outcomes, &troubleshootv1beta2.Outcome{
					Pass: &troubleshootv1beta2.SingleOutcome{
						When:    "connected",
						Message: "Successful TCP connection to 10.0.0.1:6443.",
					},
				})
			},
		},
		"TCPConnect-1": {
			match: func(hc *troubleshootv1beta2.HostAnalyze) bool {
				return hc.TCPConnect != nil && strings.HasPrefix(hc.TCPConnect.CollectorName, "tcp-connect-1")
			},
			validate: func(hc *troubleshootv1beta2.HostAnalyze) {
				assert.Contains(t, hc.TCPConnect.Outcomes, &troubleshootv1beta2.Outcome{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "connection-refused",
						Message: "A TCP connection to 10.0.0.1:9443 is required, but the connection was refused. This can occur, for example, if IP routing is not possible between this host and 10.0.0.1:9443, or if your firewall doesn't allow traffic between this host and 10.0.0.1:9443.",
					},
				})
				assert.Contains(t, hc.TCPConnect.Outcomes, &troubleshootv1beta2.Outcome{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "connection-timeout",
						Message: "A TCP connection to 10.0.0.1:9443 is required, but the connection timed out. This can occur, for example, if IP routing is not possible between this host and 10.0.0.1:9443, or if your firewall doesn't allow traffic between this host and 10.0.0.1:9443.",
					},
				})
				assert.Contains(t, hc.TCPConnect.Outcomes, &troubleshootv1beta2.Outcome{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "error",
						Message: "A TCP connection to 10.0.0.1:9443 is required, but an unexpected error occurred. This can occur, for example, if IP routing is not possible between this host and 10.0.0.1:9443, or if your firewall doesn't allow traffic between this host and 10.0.0.1:9443.",
					},
				})
				assert.Contains(t, hc.TCPConnect.Outcomes, &troubleshootv1beta2.Outcome{
					Pass: &troubleshootv1beta2.SingleOutcome{
						When:    "connected",
						Message: "Successful TCP connection to 10.0.0.1:9443.",
					},
				})
			},
		},
	})

	// --- validate commands --- //
	// Get expected hostname to validate it's included in the kubelet args
	expectedHostname, err := nodeutil.GetHostname("")
	require.NoError(t, err, "should be able to get hostname")

	assertCommands(t, dr.Commands,
		[]interface{}{
			"firewall-cmd --info-zone ec-net",
			"firewall-cmd --add-source 10.2.0.0/17 --permanent --zone ec-net",
			"firewall-cmd --add-source 10.2.128.0/17 --permanent --zone ec-net",
			"firewall-cmd --reload",
			regexp.MustCompile(fmt.Sprintf(`k0s install .* --kubelet-extra-args --node-ip=.* --hostname-override=%s`, regexp.QuoteMeta(expectedHostname))),
		},
		false,
	)

	// --- validate k0s cluster config --- //
	k0sConfig := readK0sConfig(t)

	assert.Equal(t, "10.2.0.0/17", k0sConfig.Spec.Network.PodCIDR)
	assert.Equal(t, "10.2.128.0/17", k0sConfig.Spec.Network.ServiceCIDR)
	assert.Equal(t, "192.168.1.60", k0sConfig.Spec.API.Address) // Ip should be the one associated with the inferface we defined above

	// --- validate metrics --- //
	assertMetrics(t, dr.Metrics, []struct {
		title    string
		validate func(string)
	}{
		{
			title: "JoinStarted",
			validate: func(payload string) {
				assert.Contains(t, payload, `"entryCommand":"join"`)
				assert.Regexp(t, `"flags":"--yes"`, payload)
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `"eventType":"JoinStarted"`)
			},
		},
		{
			title: "JoinSucceeded",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":true`)
				assert.Contains(t, payload, `"eventType":"JoinSucceeded"`)
			},
		},
	})

	// --- validate installation object --- //
	in, err := kubeutils.GetLatestInstallation(context.TODO(), kcli)
	if err != nil {
		t.Fatalf("failed to get latest installation: %v", err)
	}

	assert.Equal(t, clusterID.String(), in.Spec.ClusterID)
	if hasHAMigration {
		assert.True(t, in.Spec.HighAvailability, "HA should be true")
	} else {
		assert.False(t, in.Spec.HighAvailability, "HA should be false")
	}

	hcli.AssertExpectations(t)

	// --- validate admin console values --- //
	if hasHAMigration {
		var adminConsoleValues map[string]interface{}
		hcli.AssertCalled(t, "Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			if opts.ReleaseName == "admin-console" {
				adminConsoleValues = opts.Values
				return true
			}
			return false
		}))
		assertHelmValues(t, adminConsoleValues, map[string]interface{}{
			"embeddedClusterID": clusterID.String(),
		})
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestJoinRunPreflights(t *testing.T) {
	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	client := &dryrun.Client{
		Kotsadm: dryrun.NewKotsadm(),
	}
	clusterID := uuid.New()
	jcmd := &join.JoinCommandResponse{
		K0sJoinCommand:         "/usr/local/bin/k0s install controller --enable-worker --no-taints --labels kots.io/embedded-cluster-role=total-1,kots.io/embedded-cluster-role-0=controller-test,controller-label=controller-label-value",
		K0sToken:               "some-k0s-token",
		EmbeddedClusterVersion: "v0.0.0",
		ClusterID:              clusterID,
		InstallationSpec: ecv1beta1.InstallationSpec{
			ClusterID: clusterID.String(),
			Config: &ecv1beta1.ConfigSpec{
				UnsupportedOverrides: ecv1beta1.UnsupportedOverrides{},
			},
		},
		TCPConnectionsRequired: []string{"10.0.0.1:6443", "10.0.0.1:9443"},
	}
	client.Kotsadm.SetGetJoinTokenResponse("10.0.0.1", "some-token", jcmd, nil)
	dryrun.Init(drFile, client)
	dryrunJoin(t, "run-preflights", "10.0.0.1", "some-token")
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestJoinWorkerNode(t *testing.T) {
	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	client := &dryrun.Client{
		Kotsadm: dryrun.NewKotsadm(),
	}
	clusterID := uuid.New()
	jcmd := &join.JoinCommandResponse{
		K0sJoinCommand:         "/usr/local/bin/k0s install worker --no-taints --labels kots.io/embedded-cluster-role=total-1,kots.io/embedded-cluster-role-0=worker-test,worker-label=worker-label-value",
		K0sToken:               "some-k0s-token",
		EmbeddedClusterVersion: "v0.0.0",
		ClusterID:              clusterID,
		InstallationSpec: ecv1beta1.InstallationSpec{
			ClusterID: clusterID.String(),
			Config: &ecv1beta1.ConfigSpec{
				UnsupportedOverrides: ecv1beta1.UnsupportedOverrides{
					K0s: `
config:
  metadata:
    name: foo
  spec:
    telemetry:
    enabled: false
    workerProfiles:
    - name: ip-forward
    values:
      allowedUnsafeSysctls:
      - net.ipv4.ip_forward`,
				},
			},
		},
	}
	client.Kotsadm.SetGetJoinTokenResponse("10.0.0.1", "some-token", jcmd, nil)
	dryrun.Init(drFile, client)
	dr := dryrunJoin(t, "10.0.0.1", "some-token")

	// --- validate os env --- //
	assertEnv(t, dr.OSEnv, map[string]string{
		"KUBECONFIG": "/var/lib/embedded-cluster/k0s/kubelet.conf", // uses kubelet config
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
