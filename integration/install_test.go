package integration

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/replicatedhq/helmvm/integration/cluster"
)

type buffer struct {
	*bytes.Buffer
}

func (b *buffer) Close() error {
	return nil
}

func TestSingleNodeInstallation(t *testing.T) {
	t.Parallel()
	tmpcluster := cluster.NewTestCluster(&cluster.Input{
		T:             t,
		Nodes:         1,
		SSHPublicKey:  "output/tmp/id_rsa.pub",
		SSHPrivateKey: "output/tmp/id_rsa",
		HelmVMPath:    "output/bin/helmvm",
	})
	defer tmpcluster.Destroy()
	out := &buffer{bytes.NewBuffer(nil)}
	cmd := cluster.Command{
		Node:   tmpcluster.Nodes[0],
		Line:   []string{"apt", "install", "openssh-server", "-y"},
		Stdout: out,
		Stderr: out,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	t.Logf("installing ssh on node %s", tmpcluster.Nodes[0])
	if err := cluster.Run(ctx, t, cmd); err != nil {
		fmt.Printf("out: %s\n", out.String())
		t.Fatal(err)
	}
	out = &buffer{bytes.NewBuffer(nil)}
	cmd = cluster.Command{
		Node:   tmpcluster.Nodes[0],
		Line:   []string{"single-node-install.sh"},
		Stdout: out,
		Stderr: out,
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	t.Logf("installing helmvm on node %s", tmpcluster.Nodes[0])
	if err := cluster.Run(ctx, t, cmd); err != nil {
		fmt.Printf("out: %s\n", out.String())
		fmt.Printf("err: %v\n", err)
	}
}

func TestMultiNodeInstallation(t *testing.T) {
	t.Parallel()
	t.Log("creating cluster")
	tmpcluster := cluster.NewTestCluster(&cluster.Input{
		T:             t,
		Nodes:         3,
		SSHPublicKey:  "output/tmp/id_rsa.pub",
		SSHPrivateKey: "output/tmp/id_rsa",
		HelmVMPath:    "output/bin/helmvm",
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
			fmt.Printf("out: %s\n", out.String())
			t.Fatal(err)
		}
	}
	t.Logf("running install from node %s", tmpcluster.Nodes[0])
	out := &buffer{bytes.NewBuffer(nil)}
	cmd := cluster.Command{
		Node:   tmpcluster.Nodes[0],
		Line:   []string{"multi-node-install.sh"},
		Stdout: out,
		Stderr: out,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := cluster.Run(ctx, t, cmd); err != nil {
		fmt.Printf("out: %s\n", out.String())
		fmt.Printf("err: %v\n", err)
	}
}
