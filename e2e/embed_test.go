package e2e

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/replicatedhq/helmvm/e2e/cluster"
)

func TestEmbedAndInstall(t *testing.T) {
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
		Stdout: stdout,
		Stderr: stderr,
		Line:   []string{"embed-and-install.sh"},
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	t.Logf("pulling helm chart, embedding, and installing on node %s", tmpcluster.Nodes[0])
	if err := cluster.Run(ctx, t, cmd); err != nil {
		t.Errorf("stdout:\n%s\n", stdout.String())
		t.Errorf("stderr:\n%s\n", stderr.String())
		t.Fatal(err)
	}
}

func TestInstallSingleNodeAndUpgradeToEmbed(t *testing.T) {
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
	stdout = &buffer{bytes.NewBuffer(nil)}
	stderr = &buffer{bytes.NewBuffer(nil)}
	cmd = cluster.Command{
		Node:   tmpcluster.Nodes[0],
		Line:   []string{"embed-and-install.sh"},
		Stdout: stdout,
		Stderr: stderr,
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	t.Logf("installing helmvm embed with memcached on node %s", tmpcluster.Nodes[0])
	if err := cluster.Run(ctx, t, cmd); err != nil {
		t.Logf("stdout:\n%s", stdout.String())
		t.Logf("stderr:\n%s", stderr.String())
		t.Errorf("fail to deploy: %v", err)
	}
}
