package airgapbundle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// BuildRuntimeAssets materializes the resolved plan into the two production EC
// runtime archives. It verifies that registry content still matches the plan.
func BuildRuntimeAssets(ctx context.Context, plan *RuntimeManifest, chartFiles []string, outputDir string) error {
	if plan.Schema != RuntimeCacheSchema || plan.PlanDigest == "" {
		return fmt.Errorf("invalid runtime plan")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	layoutDir, err := os.MkdirTemp("", "ec-runtime-oci-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(layoutDir)
	p, err := layout.Write(layoutDir, empty.Index)
	if err != nil {
		return err
	}
	platform, err := v1.ParsePlatform(plan.Platform)
	if err != nil {
		return fmt.Errorf("parse platform: %w", err)
	}
	plan.SavedImages = nil
	for _, requested := range plan.RequestedImages {
		ref, err := name.NewDigest(requested.Repository+"@"+requested.Digest, name.WeakValidation)
		if err != nil {
			return err
		}
		desc, err := remote.Head(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
		if err != nil {
			return fmt.Errorf("verify %s: %w", requested.Reference, err)
		}
		if desc.Digest.String() != requested.Digest {
			return fmt.Errorf("resolved digest changed for %s", requested.Reference)
		}
		img, err := remote.Image(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithPlatform(*platform))
		if err != nil {
			return fmt.Errorf("copy %s: %w", requested.Reference, err)
		}
		actual, err := img.Digest()
		if err != nil {
			return err
		}
		size, err := img.Size()
		if err != nil {
			return err
		}
		annotations := map[string]string{ocispec.AnnotationRefName: requested.Reference}
		if err := p.AppendImage(img, layout.WithAnnotations(annotations)); err != nil {
			return err
		}
		plan.SavedImages = append(plan.SavedImages, SavedImage{Name: requested.Reference, Digest: actual.String(), Size: size, Platforms: []string{platform.String()}})
	}
	if err := WriteDeterministicTar(layoutDir, filepath.Join(outputDir, "images-amd64.tar")); err != nil {
		return err
	}
	chartsDir, err := os.MkdirTemp("", "ec-runtime-charts-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(chartsDir)
	for _, source := range chartFiles {
		if err := copyFile(source, filepath.Join(chartsDir, filepath.Base(source)), 0o644); err != nil {
			return err
		}
	}
	return WriteDeterministicTarGz(chartsDir, filepath.Join(outputDir, "charts.tar.gz"))
}
