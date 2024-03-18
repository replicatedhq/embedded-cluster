package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// DockerConfig represents the content of the '.dockerconfigjson' secret.
type DockerConfig struct {
	Auths map[string]DockerConfigEntry `json:"auths"`
}

// DockerConfigEntry represents the content of the '.dockerconfigjson' secret.
type DockerConfigEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

// registryAuth returns the authentication store to be used when reaching the
// registry. The authentication store is read from the cluster secret named
// 'registry-creds' in the 'kotsadm' namespace.
func registryAuth(ctx context.Context) (credentials.Store, error) {
	nsn := types.NamespacedName{Name: "registry-creds", Namespace: "kotsadm"}
	var sct corev1.Secret
	if err := kubecli.Get(ctx, nsn, &sct); err != nil {
		return nil, fmt.Errorf("unable to get secret: %w", err)
	}

	data, ok := sct.Data[".dockerconfigjson"]
	if !ok {
		return nil, fmt.Errorf("unable to find secret .dockerconfigjson")
	}

	var cfg DockerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal secret: %w", err)
	}

	creds := credentials.NewMemoryStore()
	for addr, entry := range cfg.Auths {
		creds.Put(ctx, addr, auth.Credential{
			Username: entry.Username,
			Password: entry.Password,
		})
	}
	return creds, nil
}

// pullArtifact fetches an artifact from the registry pointed by 'from'. The artifact
// is stored in a temporary directory and the path to this directory is returned.
// Callers are responsible for removing the temporary directory when it is no longer
// needed. In case of error, the temporary directory is removed here.
func pullArtifact(ctx context.Context, from string) (string, error) {
	store, err := registryAuth(ctx)
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

	// setup the pull options. XXX we are using a registry over http.
	repo.Client = &auth.Client{Credential: store.Get}
	repo.PlainHTTP = true

	tag := imgref.Reference
	if _, err := oras.Copy(ctx, repo, tag, fs, tag, oras.DefaultCopyOptions); err != nil {
		os.RemoveAll(tmpdir)
		return "", fmt.Errorf("unable to copy: %w", err)
	}
	return tmpdir, nil
}
