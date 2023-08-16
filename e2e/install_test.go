package e2e

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/replicatedhq/helmvm/e2e/cluster"
)

func TestSingleNodeInstallation(t *testing.T) {
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
		Line:   []string{"apt", "install", "openssh-server", "-y"},
		Stdout: stdout,
		Stderr: stderr,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := cluster.Run(ctx, t, cmd); err != nil {
		t.Logf("fail to install ssh on node %s: %v", tmpcluster.Nodes[0], err)
		t.Fatal(err)
	}
	stdout = &buffer{bytes.NewBuffer(nil)}
	stderr = &buffer{bytes.NewBuffer(nil)}
	cmd = cluster.Command{
		Node:   tmpcluster.Nodes[0],
		Line:   []string{"single-node-install.sh"},
		Stdout: stdout,
		Stderr: stderr,
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	t.Logf("installing helmvm on node %s", tmpcluster.Nodes[0])
	if err := cluster.Run(ctx, t, cmd); err != nil {
		t.Logf("stdout:\n%s", stdout.String())
		t.Logf("stderr:\n%s", stderr.String())
		t.Errorf("fail to deploy: %v", err)
	}
}

func TestMultiNodeInstallation(t *testing.T) {
	t.Parallel()
	t.Log("creating cluster")
	tmpcluster := cluster.NewTestCluster(&cluster.Input{
		T:             t,
		Nodes:         3,
		Image:         "ubuntu/jammy",
		SSHPublicKey:  "../output/tmp/id_rsa.pub",
		SSHPrivateKey: "../output/tmp/id_rsa",
		HelmVMPath:    "../output/bin/helmvm",
	})
	defer tmpcluster.Destroy()
	for _, node := range tmpcluster.Nodes {
		t.Logf("installing ssh on node %s", node)
		out := &buffer{bytes.NewBuffer(nil)}
		cmd := cluster.Command{
			Node:   node,
			Line:   []string{"apt", "install", "openssh-server", "-y"},
			Stdout: out,
			Stderr: out,
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := cluster.Run(ctx, t, cmd); err != nil {
			t.Logf("fail to install ssh on node %s: %v", node, err)
			t.Fatal(err)
		}
	}
	t.Logf("running helmvm install from node %s", tmpcluster.Nodes[0])
	stdout := &buffer{bytes.NewBuffer(nil)}
	stderr := &buffer{bytes.NewBuffer(nil)}
	cmd := cluster.Command{
		Node:   tmpcluster.Nodes[0],
		Line:   []string{"multi-node-install.sh"},
		Stdout: stdout,
		Stderr: stderr,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := cluster.Run(ctx, t, cmd); err != nil {
		t.Logf("stdout:\n%s", stdout.String())
		t.Logf("stderr:\n%s", stderr.String())
		t.Errorf("fail to deploy: %s", err)
	}
}
