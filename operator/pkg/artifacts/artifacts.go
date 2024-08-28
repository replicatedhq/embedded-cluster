package artifacts

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"github.com/go-logr/logr"
	"go.uber.org/multierr"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Pull fetches an artifact from the registry pointed by 'from'. The artifact is stored in a temporary
// directory and the path to this directory is returned. Callers are responsible for removing the temp
// path when it is no longer needed. In case of error, the temporary directory is removed here.
func Pull(ctx context.Context, log logr.Logger, cli client.Client, from string) (string, error) {
	log.Info("Reading registry credentials from cluster")
	store, err := registryAuth(ctx, log, cli)
	if err != nil {
		return "", fmt.Errorf("unable to get registry auth: %w", err)
	}

	log.Info("Pulling artifact from registry", "from", from)
	imgref, err := registry.ParseReference(from)
	if err != nil {
		return "", fmt.Errorf("unable to parse image reference: %w", err)
	}

	tmpdir, err := os.MkdirTemp("", "embedded-cluster-artifact-*")
	if err != nil {
		return "", fmt.Errorf("unable to create temp dir: %w", err)
	}

	repo, err := remote.NewRepository(from)
	if err != nil {
		return "", fmt.Errorf("unable to create repository: %w", err)
	}

	fs, err := file.New(tmpdir)
	if err != nil {
		return "", fmt.Errorf("unable to create file store: %w", err)
	}
	defer fs.Close()

	transp, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return "", fmt.Errorf("unable to get default transport")
	}

	transp = transp.Clone()
	transp.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	repo.Client = &auth.Client{
		Client:     &http.Client{Transport: transp},
		Credential: store.Get,
	}

	tag := imgref.Reference
	_, tlserr := oras.Copy(ctx, repo, tag, fs, tag, oras.DefaultCopyOptions)
	if tlserr == nil {
		return tmpdir, nil
	}

	// if we fail to fetch the artifact using https we gonna try once more using plain
	// http as some versions of the registry were deployed without tls.
	repo.PlainHTTP = true
	log.Info("Unable to fetch artifact using tls, retrying with http")
	if _, err := oras.Copy(ctx, repo, tag, fs, tag, oras.DefaultCopyOptions); err != nil {
		os.RemoveAll(tmpdir)
		err = multierr.Combine(tlserr, err)
		return "", fmt.Errorf("unable to fetch artifacts with or without tls: %w", err)
	}
	return tmpdir, nil
}
