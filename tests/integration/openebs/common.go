package openebs

import (
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/tests/integration/openebs/static"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
)

func createPodAndPVC(t *testing.T, kubeconfig string) {
	// create a Pod and PVC to test that the data dir is mounted
	b, err := static.FS.ReadFile("podandpvc.yaml")
	if err != nil {
		t.Fatalf("failed to read podandpvc.yaml: %s", err)
	}
	filename := util.WriteTempFile(t, "podandpvc-*.yaml", b, 0644)
	util.KubectlApply(t, kubeconfig, "default", filename)
	util.WaitForPodComplete(t, kubeconfig, "default", "task-pv-pod", 2*time.Minute)
}
