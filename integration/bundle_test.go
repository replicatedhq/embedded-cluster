package integration

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/replicatedhq/helmvm/integration/cluster"
)

func TestBuildBundle(t *testing.T) {
	t.Parallel()
	tmpcluster := cluster.NewTestCluster(&cluster.Input{
		T:             t,
		Nodes:         1,
		SSHPublicKey:  "/root/.ssh/id_rsa.pub",
		SSHPrivateKey: "/root/.ssh/id_rsa",
		HelmVMPath:    "/usr/local/bin/helmvm",
	})
	defer tmpcluster.Destroy()
	out := &buffer{bytes.NewBuffer(nil)}
	cmd := cluster.Command{
		Node:   tmpcluster.Nodes[0],
		Line:   []string{"helmvm", "build-bundle"},
		Stdout: out,
		Stderr: out,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	t.Logf("building helmvm bundle in node %s", tmpcluster.Nodes[0])
	if err := cluster.Run(ctx, t, cmd); err != nil {
		fmt.Printf("out: %s\n", out.String())
		fmt.Printf("err: %v\n", err)
	}
}
