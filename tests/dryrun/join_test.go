package dryrun

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestJoinTCPConnectionsRequired(t *testing.T) {
	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	client := &dryrun.Client{
		Kotsadm: dryrun.NewKotsadm(),
	}
	clusterID := uuid.New()
	jcmd := &kotsadm.JoinCommandResponse{
		K0sJoinCommand:         "/usr/local/bin/k0s install controller --enable-worker --no-taints --labels kots.io/embedded-cluster-role=total-1,kots.io/embedded-cluster-role-0=controller-test,controller-label=controller-label-value",
		K0sToken:               "some-k0s-token",
		EmbeddedClusterVersion: "v0.0.0",
		ClusterID:              clusterID,
		InstallationSpec: ecv1beta1.InstallationSpec{
			ClusterID: clusterID.String(),
			Config: &ecv1beta1.ConfigSpec{
				UnsupportedOverrides: ecv1beta1.UnsupportedOverrides{},
			},
			Network: &ecv1beta1.NetworkSpec{
				PodCIDR:     "10.2.0.0/17",
				ServiceCIDR: "10.2.128.0/17",
			},
		},
		TCPConnectionsRequired: []string{"10.0.0.1:6443", "10.0.0.1:9443"},
	}
	client.Kotsadm.SetGetJoinTokenResponse("10.0.0.1", "some-token", jcmd, nil)
	dryrun.Init(drFile, client)
	dr := dryrunJoin(t, "10.0.0.1", "some-token")

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

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestJoinRunPreflights(t *testing.T) {
	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	client := &dryrun.Client{
		Kotsadm: dryrun.NewKotsadm(),
	}
	clusterID := uuid.New()
	jcmd := &kotsadm.JoinCommandResponse{
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
	jcmd := &kotsadm.JoinCommandResponse{
		K0sJoinCommand:         "/usr/local/bin/k0s install worker --no-taints --labels kots.io/embedded-cluster-role=total-1,kots.io/embedded-cluster-role-0=worker-test,worker-label=worker-label-value",
		K0sToken:               "some-k0s-token",
		EmbeddedClusterVersion: "v0.0.0",
		ClusterID:              clusterID,
		InstallationSpec: ecv1beta1.InstallationSpec{
			ClusterID: clusterID.String(),
			Config: &ecv1beta1.ConfigSpec{
				UnsupportedOverrides: ecv1beta1.UnsupportedOverrides{},
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
