// Package release contains function to help finding things out about a given
// embedded cluster release. It is being kept here so if we decide to manage
// releases in a different way, we can easily change it.
package release

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gosimple/slug"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	metaURL = "%s/embedded-cluster-public-files/metadata/v%s.json"
	cache   = map[string]*Meta{}
	mutex   = sync.Mutex{}
)

// Versions holds a list of add-on versions.
type Versions struct {
	AdminConsole            string
	EmbeddedClusterOperator string
	Installer               string
	Kubernetes              string
	OpenEBS                 string
}

// Meta represents the components of a given embedded cluster release. This
// is read directly from GitHub releases page.
type Meta struct {
	Versions  Versions
	K0sSHA    string
	Configs   *k0sv1beta1.HelmExtensions
	Protected map[string][]string
}

// LocalVersionMetadataConfigmap returns the namespaced name for a config map name that contains
// the metadata for a given embedded cluster version.
func LocalVersionMetadataConfigmap(version string) types.NamespacedName {
	version = slug.Make(strings.TrimPrefix(version, "v"))
	return types.NamespacedName{
		Name:      fmt.Sprintf("version-metadata-%s", version),
		Namespace: "embedded-cluster",
	}
}

// MetadataFor determines from where to read the metadata (from the cluster or remotely) and calls
// the appropriate function.
func MetadataFor(ctx context.Context, in *v1beta1.Installation, cli client.Client) (*Meta, error) {
	if in.Spec.AirGap {
		return localMetadataFor(ctx, cli, in.Spec.Config.Version)
	}
	return remoteMetadataFor(ctx, in.Spec.Config.Version, in.Spec.MetricsBaseURL)
}

// localMetadataFor reads metadata for a given release. Attempts to read a local config map.
func localMetadataFor(ctx context.Context, cli client.Client, version string) (*Meta, error) {
	mutex.Lock()
	defer mutex.Unlock()

	version = strings.TrimPrefix(version, "v")
	if _, ok := cache[version]; ok {
		return metaFromCache(version)
	}

	var cm corev1.ConfigMap
	nsn := LocalVersionMetadataConfigmap(version)
	if err := cli.Get(ctx, nsn, &cm); err != nil {
		return nil, fmt.Errorf("failed to get config map %q: %w", nsn.Name, err)
	}

	data, ok := cm.Data["metadata.json"]
	if !ok {
		return nil, fmt.Errorf("metadata.json not found in config map %q", nsn.Name)
	}

	var meta Meta
	if err := json.Unmarshal([]byte(data), &meta); err != nil {
		return nil, fmt.Errorf("failed to decode bundle: %w", err)
	}
	cache[version] = &meta
	return metaFromCache(version)
}

// remoteMetadataFor reads metadata for a given release. Goes to replicated.app and reads release metadata file
func remoteMetadataFor(ctx context.Context, version string, upstream string) (*Meta, error) {
	mutex.Lock()
	defer mutex.Unlock()
	version = strings.TrimPrefix(version, "v")
	if _, ok := cache[version]; ok {
		return metaFromCache(version)
	}
	url := fmt.Sprintf(metaURL, upstream, version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get bundle from %q: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get bundle from %q: %s", url, resp.Status)
	}
	var meta Meta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode bundle: %w", err)
	}
	cache[version] = &meta
	return metaFromCache(version)
}

// CacheMeta caches a given meta for a given version. It is intended for unit testing.
func CacheMeta(version string, meta Meta) {
	mutex.Lock()
	defer mutex.Unlock()
	cache[version] = &meta
}

// metaFromCache returns a version from the cache, but without any pointers that might update things still in the cache.
func metaFromCache(version string) (*Meta, error) {
	// take the cached version and turn it into json
	meta := cache[version]
	if meta == nil {
		return nil, nil
	}
	stringVer, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal meta: %w", err)
	}

	returnVersion := Meta{}
	// unmarshal the json back into a Meta struct
	err = json.Unmarshal(stringVer, &returnVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal meta: %w", err)
	}

	return &returnVersion, nil
}
