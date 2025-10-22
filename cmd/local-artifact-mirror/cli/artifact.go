package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/artifacts"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PullArtifactFunc func(ctx context.Context, kcli client.Client, from string) (string, error)

func pullArtifact(ctx context.Context, kcli client.Client, from string) (string, error) {
	tmpdir, err := os.MkdirTemp("", "lam-artifact-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	opts := artifacts.PullOptions{}
	err = artifacts.Pull(ctx, kcli, from, tmpdir, opts)
	if err == nil {
		return tmpdir, nil
	}

	// if we fail to fetch the artifact using https we gonna try once more using plain
	// http as some versions of the registry were deployed without tls.
	opts.PlainHTTP = true
	if err := artifacts.Pull(ctx, kcli, from, tmpdir, opts); err == nil {
		return tmpdir, nil
	}

	os.RemoveAll(tmpdir)
	return "", fmt.Errorf("pull artifact: %w", err)
}
