package e2e

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestVersion(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "ubuntu/jammy",
		SSHPublicKey:        "../output/tmp/id_rsa.pub",
		SSHPrivateKey:       "../output/tmp/id_rsa",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Destroy()
	t.Logf("%s: validating embedded-cluster version in node 0", time.Now().Format(time.RFC3339))
	line := []string{"embedded-cluster", "version"}
	stdout, stderr, err := RunCommandOnNode(t, tc, 0, line)
	if err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
	var failed bool
	output := fmt.Sprintf("%s\n%s", stdout, stderr)
	expected := []string{"Installer", "Kubernetes", "OpenEBS", "AdminConsole", "EmbeddedClusterOperator", "memcached-vendor-chart", "embedded-cluster"}
	for _, component := range expected {
		if strings.Contains(output, component) {
			continue
		}
		t.Errorf("missing %q version in 'version' output", component)
		failed = true
	}
	if failed {
		t.Log(output)
		return
	}

	line2 := []string{"embedded-cluster", "version", "metadata"}
	stdout, stderr, err = RunCommandOnNode(t, tc, 0, line2)
	if err != nil {
		t.Fatalf("fail to run metadata command on node %s: %v", tc.Nodes[0], err)
	}

	output = fmt.Sprintf("%s\n%s", stdout, stderr)
	parsed := struct {
		Configs k0sconfig.HelmExtensions
	}{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Log(output)
		t.Fatalf("fail to parse metadata output: %v", err)
	}

	expectedCharts := []string{"openebs", "embedded-cluster-operator", "admin-console", "memcached-vendor-chart"}
	if len(parsed.Configs.Charts) != len(expectedCharts) {
		t.Log(output)
		t.Fatalf("found %d charts in metadata, expected %d", len(parsed.Configs.Charts), len(expectedCharts))
	}

	for _, expectedName := range expectedCharts {
		foundName := false
		for _, foundChart := range parsed.Configs.Charts {
			if foundChart.Name == expectedName {
				foundName = true
				break
			}
		}
		if !foundName {
			t.Errorf("failed to find chart %s in 'metadata' output", expectedName)
			failed = true
		}
	}

	if failed {
		t.Log(output)
		return
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
