package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// registryAuth returns the credentials to reach the registry. These credentials are
// read from the cluster. XXX It is still not defined the secret name from where we
// are going to read the credentials so this returns empty credentials instead.
func registryAuth(_ context.Context) (auth.Credential, error) {
	return auth.Credential{}, nil
}

// pullArtifact fetches an artifact from the registry pointed by 'from'. The artifact
// is stored in a temporary directory and the path to this directory is returned.
// Callers are responsible for removing the temporary directory when it is no longer
// needed. In case of error, the temporary directory is removed here.
func pullArtifact(ctx context.Context, from string) (string, error) {
	creds, err := registryAuth(ctx)
	if err != nil {
		return "", fmt.Errorf("unable to get registry auth: %w", err)
	}

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

	// XXX for now we are using a custom transport to skip the certificate validation
	//     because the test environment is using a self-signed certificate. This should
	//     be changed.
	repo.Client = &auth.Client{
		Credential: auth.StaticCredential(imgref.Registry, creds),
		Client: &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
	}

	tag := imgref.Reference
	if _, err := oras.Copy(ctx, repo, tag, fs, tag, oras.DefaultCopyOptions); err != nil {
		os.RemoveAll(tmpdir)
		return "", fmt.Errorf("unable to copy: %w", err)
	}
	return tmpdir, nil
}
