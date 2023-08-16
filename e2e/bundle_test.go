package e2e

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/replicatedhq/helmvm/e2e/cluster"
)

func TestBuildBundle(t *testing.T) {
	t.Parallel()
	tmpcluster := cluster.NewTestCluster(&cluster.Input{
		T:             t,
		Nodes:         1,
		Image:         "ubuntu/jammy",
		SSHPublicKey:  "../output/tmp/id_rsa.pub",
		SSHPrivateKey: "../output/tmp/id_rsa",
		HelmVMPath:    "../output/bin/helmvm",
	})
	defer tmpcluster.Destroy()
	stdout := &buffer{bytes.NewBuffer(nil)}
	stderr := &buffer{bytes.NewBuffer(nil)}
	cmd := cluster.Command{
		Node:   tmpcluster.Nodes[0],
		Line:   []string{"build-bundle.sh"},
		Stdout: stdout,
		Stderr: stderr,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	t.Logf("building helmvm bundle in node %s", tmpcluster.Nodes[0])
	if err := cluster.Run(ctx, t, cmd); err != nil {
		t.Logf("stdout:\n%s", stdout.String())
		t.Logf("stderr:\n%s", stderr.String())
		t.Errorf("fail to build bundle: %v", err)
	}
}
