package artifacts

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/go-logr/logr"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DockerConfig represents the content of the '.dockerconfigjson' secret.
type DockerConfig struct {
	Auths map[string]DockerConfigEntry `json:"auths"`
}

// DockerConfigEntry represents the content of the '.dockerconfigjson' secret.
type DockerConfigEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// registryAuth returns the authentication store to be used when reaching the
// registry. The authentication store is read from the cluster secret named
// 'registry-creds' in the 'kotsadm' namespace.
func registryAuth(ctx context.Context, log logr.Logger, cli client.Client) (credentials.Store, error) {
	nsn := types.NamespacedName{Name: "registry-creds", Namespace: "kotsadm"}
	var sct corev1.Secret
	if err := cli.Get(ctx, nsn, &sct); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("unable to get secret: %w", err)
		}
		log.Info("registry-creds secret not found, using anonymous access")
		return credentials.NewMemoryStore(), nil
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

// Pull fetches an artifact from the registry pointed by 'from'. The artifact is stored in a temporary
// directory and the path to this directory is returned. Callers are responsible for removing the temp
// path when it is no longer needed. In case of error, the temporary directory is removed here.
func Pull(ctx context.Context, log logr.Logger, cli client.Client, from string) (string, error) {
	log.Info("reading registry credentials from cluster")
	store, err := registryAuth(ctx, log, cli)
	if err != nil {
		return "", fmt.Errorf("unable to get registry auth: %w", err)
	}

	log.Info("pulling artifact from registry", "from", from)
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
	log.Info("unable to fetch artifact using tls, retrying with http")
	if _, err := oras.Copy(ctx, repo, tag, fs, tag, oras.DefaultCopyOptions); err != nil {
		os.RemoveAll(tmpdir)
		err = multierr.Combine(tlserr, err)
		return "", fmt.Errorf("unable to fetch artifacts with or without tls: %w", err)
	}
	return tmpdir, nil
}
