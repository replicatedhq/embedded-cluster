package airgapbundle

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplicationDigestIsStableAndRoleSpecific(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "b.yaml"), "b\n")
	mustWrite(t, filepath.Join(dir, "a.yaml"), "a\n")
	a, err := ApplicationDigest("install", dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ApplicationDigest("install", dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := ApplicationDigest("noop", dir)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("identity is not deterministic: %#v %#v", a, b)
	}
	if a.Digest != c.Digest || a.Version == c.Version {
		t.Fatal("identical inputs must share a digest but have role-specific versions")
	}
	if !strings.HasPrefix(a.Version, "airgap-e2e-install-sha") {
		t.Fatalf("unexpected version %q", a.Version)
	}
}

func TestResolveImagesDoesNotContactRegistryForPinnedReference(t *testing.T) {
	images, err := ResolveImages(context.Background(), []string{"does-not-exist.invalid/team/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if err != nil {
		t.Fatal(err)
	}
	if images[0].Digest != "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("unexpected digest %q", images[0].Digest)
	}
}

func TestSeparateConfigPreservesOtherDocuments(t *testing.T) {
	source, app := filepath.Join(t.TempDir(), "source"), filepath.Join(t.TempDir(), "app")
	mustWrite(t, filepath.Join(source, "objects.yaml"), "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app\n---\napiVersion: embeddedcluster.replicated.com/v1beta1\nkind: Config\nmetadata:\n  name: ec\n")
	config := filepath.Join(t.TempDir(), "config.yaml")
	if err := SeparateConfig(source, app, config); err != nil {
		t.Fatal(err)
	}
	application, _ := os.ReadFile(filepath.Join(app, "objects.yaml"))
	ec, _ := os.ReadFile(config)
	if strings.Contains(string(application), "embeddedcluster.replicated.com") {
		t.Fatal("EC Config remained in application")
	}
	if !strings.Contains(string(application), "ConfigMap") || !strings.Contains(string(ec), "kind: Config") {
		t.Fatal("documents were not separated")
	}
}

func TestRuntimePlanNormalizesInputsAndVerifiesFiles(t *testing.T) {
	images := []RequestedImage{{Reference: "b:tag", Repository: "b", Digest: "sha256:2"}, {Reference: "a@sha256:1", Repository: "a", Digest: "sha256:1"}, {Reference: "duplicate", Repository: "a", Digest: "sha256:1"}}
	one, err := NewRuntimePlan("airgap", "linux/amd64", images, []Chart{{Name: "z", Digest: "sha256:9"}})
	if err != nil {
		t.Fatal(err)
	}
	two, err := NewRuntimePlan("airgap", "linux/amd64", []RequestedImage{images[2], images[0], images[1]}, []Chart{{Name: "z", Digest: "sha256:9"}})
	if err != nil {
		t.Fatal(err)
	}
	if one.PlanDigest != two.PlanDigest || len(one.RequestedImages) != 2 {
		t.Fatal("plan was not sorted and deduplicated")
	}
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "images-amd64.tar"), "images")
	mustWrite(t, filepath.Join(dir, "charts.tar.gz"), "charts")
	if err := one.Complete(filepath.Join(dir, "images-amd64.tar"), filepath.Join(dir, "charts.tar.gz")); err != nil {
		t.Fatal(err)
	}
	if err := one.Write(filepath.Join(dir, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	if err := one.Verify(dir); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "charts.tar.gz"), "changed")
	if err := one.Verify(dir); err == nil {
		t.Fatal("expected checksum failure")
	}
}

func TestBuildRuntimeAssetsWithoutImages(t *testing.T) {
	chart := filepath.Join(t.TempDir(), "test-1.0.0.tgz")
	mustWrite(t, chart, "chart")
	plan, err := NewRuntimePlan("online", "linux/amd64", nil, []Chart{{Name: filepath.Base(chart), Digest: "sha256:test"}})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "runtime")
	if err := BuildRuntimeAssets(context.Background(), &plan, []string{chart}, out); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"images-amd64.tar", "charts.tar.gz"} {
		if info, err := os.Stat(filepath.Join(out, name)); err != nil || info.Size() == 0 {
			t.Fatalf("missing archive %s", name)
		}
	}
}

func TestAugmentUsesProductionV2Layout(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	mustWrite(t, filepath.Join(appDir, "application.yaml"), "kind: Application\n")
	mustWrite(t, filepath.Join(appDir, "airgap.yaml"), "apiVersion: kots.io/v1beta1\nkind: Airgap\nspec:\n  appSlug: test\n")
	appBundle := filepath.Join(root, "app.airgap")
	if err := WriteDeterministicTarGz(appDir, appBundle); err != nil {
		t.Fatal(err)
	}
	ec := filepath.Join(root, "ec")
	for _, name := range []string{"embedded-cluster-amd64", "version-metadata.json", "images-amd64.tar", "charts.tar.gz", "artifacts/kots.tar.gz", "registry/docker/registry/v2/repositories/x"} {
		mustWrite(t, filepath.Join(ec, name), name)
	}
	output := filepath.Join(root, "complete.airgap")
	if err := Augment(appBundle, ec, "final-version", output); err != nil {
		t.Fatal(err)
	}
	extracted := filepath.Join(root, "extracted")
	if err := extractTarGz(output, extracted); err != nil {
		t.Fatal(err)
	}
	metadataData, err := os.ReadFile(filepath.Join(extracted, "airgap.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metadataData), "versionLabel: final-version") {
		t.Fatalf("airgap metadata does not contain final version label:\n%s", metadataData)
	}
	names := archiveNames(t, output)
	for _, want := range []string{"application.yaml", "airgap.yaml", "embedded-cluster/embedded-cluster-amd64", "embedded-cluster/artifacts/kots.tar.gz", "embedded-cluster/registry/docker/registry/v2/repositories/x"} {
		if !names[want] {
			t.Errorf("missing %s", want)
		}
	}
	if _, err := os.Stat(output + ".manifest.json"); err != nil {
		t.Fatal("missing sidecar bundle manifest")
	}
}

func mustWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
}
func archiveNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	out := map[string]bool{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		out[strings.TrimSuffix(h.Name, "/")] = true
	}
	return out
}
