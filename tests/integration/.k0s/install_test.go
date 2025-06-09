package k0s

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/yaml"
)

type installTestCase struct {
	networkInterface     string
	isAirgap             bool
	podCIDR              string
	serviceCIDR          string
	domains              ecv1beta1.Domains
	vendorOverrides      *k0sv1beta1.ClusterConfig
	endUserOverridesPath string
}

func TestInstall_basic(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	testInstall(t, installTestCase{
		networkInterface:     "eth0",
		isAirgap:             false,
		podCIDR:              "10.244.0.0/16",
		serviceCIDR:          "10.96.0.0/12",
		domains:              ecv1beta1.Domains{},
		vendorOverrides:      nil,
		endUserOverridesPath: "",
	})
}

func TestInstall_workerProfile(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	testInstall(t, installTestCase{
		networkInterface: "eth0",
		isAirgap:         false,
		podCIDR:          "10.244.0.0/16",
		serviceCIDR:      "10.96.0.0/12",
		domains:          ecv1beta1.Domains{},
		vendorOverrides: &k0sv1beta1.ClusterConfig{
			Spec: &k0sv1beta1.ClusterSpec{
				WorkerProfiles: []k0sv1beta1.WorkerProfile{
					{
						Name: "max-pods",
						Config: &runtime.RawExtension{
							Raw: []byte(`{"maxPods": 1000}`),
						},
					},
				},
			},
		},
		endUserOverridesPath: "",
	})
}

func testInstall(t *testing.T, test installTestCase) {
	vendorOverridesStr := ""
	if test.vendorOverrides != nil {
		data, err := yaml.Marshal(test.vendorOverrides)
		if err != nil {
			t.Fatalf("failed to marshal vendor overrides: %s", err)
		}
		vendorOverridesStr = string(data)
	}

	t.Logf("writing k0s config")
	k0sCfg, err := k0s.WriteK0sConfig(
		t.Context(), test.networkInterface, test.isAirgap, test.podCIDR, test.serviceCIDR,
		test.domains, vendorOverridesStr, test.endUserOverridesPath,
	)
	if err != nil {
		t.Fatalf("failed to write k0s config: %s", err)
	}
	b, _ := yaml.Marshal(k0sCfg)
	t.Logf("k0s config:\n%s", string(b))

	t.Logf("installing k0s")
	err = k0s.Install(test.networkInterface)
	if err != nil {
		t.Fatalf("failed to install k0s: %s", err)
	}

	t.Logf("waiting for k0s to be ready")
	err = k0s.WaitForReady()
	if err != nil {
		t.Fatalf("failed to wait for k0s: %s", err)
	}

	os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())

	t.Logf("waiting for node to be ready")
	err = waitForNode(t.Context())
	if err != nil {
		t.Fatalf("failed to wait for node: %s", err)
	}
}

func waitForNode(ctx context.Context) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}
	nodename := strings.ToLower(hostname)
	if err := kubeutils.WaitForNode(ctx, kcli, nodename, false, &kubeutils.WaitOptions{
		Backoff: &wait.Backoff{Steps: 30, Duration: 2 * time.Second, Factor: 1.0, Jitter: 0.1}, // 1 minute
	}); err != nil {
		return fmt.Errorf("wait for node: %w", err)
	}
	return nil
}
