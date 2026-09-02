package airgapbundle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const RuntimeCacheSchema = "v1"

type RequestedImage struct {
	Reference  string `json:"reference"`
	Repository string `json:"repository"`
	Digest     string `json:"digest"`
}

type SavedImage struct {
	Name      string   `json:"name"`
	Digest    string   `json:"digest"`
	Size      int64    `json:"size"`
	Platforms []string `json:"platforms,omitempty"`
}

type Chart struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

// ResolveImages uses an already supplied digest without registry access. A
// tag-only reference is resolved with a registry HEAD request; remote.Head sets
// OCI and Docker index/manifest Accept types and performs registry auth.
func ResolveImages(ctx context.Context, references []string) ([]RequestedImage, error) {
	result := make([]RequestedImage, 0, len(references))
	for _, value := range references {
		ref, err := name.ParseReference(value, name.WeakValidation)
		if err != nil {
			return nil, fmt.Errorf("parse image %q: %w", value, err)
		}
		digest := ""
		if d, ok := ref.(name.Digest); ok {
			digest = d.DigestStr()
		} else {
			desc, err := remote.Head(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
			if err != nil {
				return nil, fmt.Errorf("resolve image %q: %w", value, err)
			}
			digest = desc.Digest.String()
		}
		repository := ref.Context().Name()
		result = append(result, RequestedImage{Reference: value, Repository: strings.TrimSuffix(repository, "/"), Digest: digest})
	}
	return result, nil
}

type RuntimeManifest struct {
	Schema          string            `json:"schema"`
	NetworkMode     string            `json:"networkMode"`
	Platform        string            `json:"platform"`
	PlanDigest      string            `json:"planDigest"`
	RequestedImages []RequestedImage  `json:"requestedImages"`
	SavedImages     []SavedImage      `json:"savedImages,omitempty"`
	Charts          []Chart           `json:"charts"`
	Files           map[string]string `json:"files,omitempty"`
}

func NewRuntimePlan(networkMode, platform string, images []RequestedImage, charts []Chart) (RuntimeManifest, error) {
	if networkMode != "online" && networkMode != "airgap" {
		return RuntimeManifest{}, fmt.Errorf("network mode must be online or airgap")
	}
	if platform == "" {
		return RuntimeManifest{}, fmt.Errorf("platform is required")
	}
	for _, image := range images {
		if image.Repository == "" || image.Digest == "" {
			return RuntimeManifest{}, fmt.Errorf("image repository and digest are required")
		}
	}
	sort.Slice(images, func(i, j int) bool {
		if images[i].Repository == images[j].Repository {
			return images[i].Digest < images[j].Digest
		}
		return images[i].Repository < images[j].Repository
	})
	images = dedupeImages(images)
	sort.Slice(charts, func(i, j int) bool {
		if charts[i].Name == charts[j].Name {
			return charts[i].Digest < charts[j].Digest
		}
		return charts[i].Name < charts[j].Name
	})
	p := RuntimeManifest{Schema: RuntimeCacheSchema, NetworkMode: networkMode, Platform: platform, RequestedImages: images, Charts: charts}
	h := sha256.New()
	_, _ = io.WriteString(h, p.Schema+"\x00"+networkMode+"\x00"+platform+"\x00")
	for _, image := range images {
		_, _ = io.WriteString(h, image.Repository+"@"+image.Digest+"\x00")
	}
	for _, chart := range charts {
		_, _ = io.WriteString(h, chart.Name+"@"+chart.Digest+"\x00")
	}
	p.PlanDigest = hex.EncodeToString(h.Sum(nil))
	return p, nil
}

func dedupeImages(in []RequestedImage) []RequestedImage {
	out := in[:0]
	for _, item := range in {
		if len(out) > 0 && out[len(out)-1].Repository == item.Repository && out[len(out)-1].Digest == item.Digest {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (m *RuntimeManifest) Complete(imagesArchive, chartsArchive string) error {
	files := map[string]string{}
	for name, path := range map[string]string{"images-amd64.tar": imagesArchive, "charts.tar.gz": chartsArchive} {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		files[name] = "sha256:" + hex.EncodeToString(h.Sum(nil))
	}
	m.Files = files
	return nil
}

func (m RuntimeManifest) Write(path string) error {
	if m.Files == nil {
		return fmt.Errorf("runtime manifest is incomplete")
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func ReadRuntimeManifest(path string) (RuntimeManifest, error) {
	var m RuntimeManifest
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}

func (m RuntimeManifest) Verify(dir string) error {
	if m.Schema != RuntimeCacheSchema || m.Files == nil {
		return fmt.Errorf("runtime cache is incomplete or has unsupported schema")
	}
	for name, expected := range m.Files {
		f, err := os.Open(dir + string(os.PathSeparator) + name)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		_ = f.Close()
		if copyErr != nil {
			return copyErr
		}
		actual := "sha256:" + hex.EncodeToString(h.Sum(nil))
		if actual != expected {
			return fmt.Errorf("%s checksum mismatch: got %s, want %s", name, actual, expected)
		}
	}
	return nil
}
