package dryrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types/join"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestJoinControllerNode(t *testing.T) {
	testJoinControllerNodeImpl(t, false)
}

func TestJoinAirgapControllerNode(t *testing.T) {
	testJoinControllerNodeImpl(t, true)
}

func testJoinControllerNodeImpl(t *testing.T, isAirgap bool) {
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
					PodCIDR:     "10.2.0.0/17",
					ServiceCIDR: "10.2.128.0/17",
				},
			},
		},
		TCPConnectionsRequired: []string{"10.0.0.1:6443", "10.0.0.1:9443"},
	}

	kotsadm := dryrun.NewKotsadm()
	kubeUtils := &dryrun.KubeUtils{}

	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	dryrun.Init(drFile, &dryrun.Client{
		Kotsadm:   kotsadm,
		KubeUtils: kubeUtils,
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

	kubeClient, err := kubeUtils.KubeClient()
	require.NoError(t, err)

	kubeClient.Create(context.Background(), &ecv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Installation",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "20241002205018",
		},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "2.2.0+k8s-1.30",
			},
			RuntimeConfig: &ecv1beta1.RuntimeConfigSpec{},
		},
	}, &ctrlclient.CreateOptions{})

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
	assertCommands(t, dr.Commands,
		[]interface{}{
			"firewall-cmd --info-zone ec-net",
			"firewall-cmd --add-source 10.2.0.0/17 --permanent --zone ec-net",
			"firewall-cmd --add-source 10.2.128.0/17 --permanent --zone ec-net",
			"firewall-cmd --reload",
		},
		false,
	)

	// --- validate k0s cluster config --- //
	k0sConfig := readK0sConfig(t)

	assert.Equal(t, "10.2.0.0/17", k0sConfig.Spec.Network.PodCIDR)
	assert.Equal(t, "10.2.128.0/17", k0sConfig.Spec.Network.ServiceCIDR)

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
