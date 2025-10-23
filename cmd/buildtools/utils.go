package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/distribution/reference"
	"github.com/google/go-github/v62/github"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/repo"
	"oras.land/oras-go/v2/registry/remote"
)

var (
	apkoLoginOnce sync.Once
)

func ApkoLogin() error {
	var retErr error
	apkoLoginOnce.Do(func() {
		cmd := exec.Command("make", "apko")
		if err := RunCommand(cmd); err != nil {
			retErr = fmt.Errorf("make apko: %w", err)
			return
		}
		if os.Getenv("IMAGES_REGISTRY_USER") != "" && os.Getenv("IMAGES_REGISTRY_PASS") != "" {
			cmd := exec.Command(
				"make",
				"apko-login",
				fmt.Sprintf("REGISTRY=%s", os.Getenv("IMAGES_REGISTRY_SERVER")),
				fmt.Sprintf("USERNAME=%s", os.Getenv("IMAGES_REGISTRY_USER")),
				fmt.Sprintf("PASSWORD=%s", os.Getenv("IMAGES_REGISTRY_PASS")),
			)
			if err := RunCommand(cmd); err != nil {
				retErr = fmt.Errorf("run make apko-login: %w", err)
				return
			}
		}
	})
	return retErr
}

func ApkoBuildAndPublish(componentName, packageName, packageVersion string, archs string) error {
	image, err := ComponentImageName(componentName, packageName, packageVersion)
	if err != nil {
		return fmt.Errorf("component image name: %w", err)
	}
	args := []string{
		"apko-build-and-publish",
		fmt.Sprintf("IMAGE=%s", image),
		fmt.Sprintf("APKO_CONFIG=%s", filepath.Join("deploy", "images", componentName, "apko.tmpl.yaml")),
		fmt.Sprintf("PACKAGE_VERSION=%s", packageVersion),
		fmt.Sprintf("ARCHS=%s", archs),
	}
	cmd := exec.Command("make", args...)
	if err := RunCommand(cmd); err != nil {
		return fmt.Errorf("run make apko-build-and-publish: %w", err)
	}
	return nil
}

func UpdateImages(ctx context.Context, imageComponents map[string]addonComponent, metaImages map[string]release.AddonImage, images []string, filteredImages []string) (map[string]release.AddonImage, error) {
	nextImages := map[string]release.AddonImage{}

	for _, image := range images {
		component, ok := imageComponents[RemoveTagFromImage(image)]
		if !ok {
			return nil, fmt.Errorf("no component found for image %s", image)
		}

		// if we have a filtered list of images, and the current image is not in the list, skip it
		// and use the image from the metadata if it exists
		if len(filteredImages) > 0 && !slices.Contains(filteredImages, component.name) {
			logrus.Infof("skipping image %s as it is not in the filtered list", image)
			if image, ok := metaImages[component.name]; ok {
				nextImages[component.name] = image
			}
			continue
		}

		newimage := metaImages[component.name]
		if newimage.Tag == nil {
			newimage.Tag = make(map[string]string)
		}

		archs := GetSupportedArchs()

		_, err := component.buildImage(ctx, image, strings.Join(archs, ","))
		if err != nil {
			return nil, fmt.Errorf("build image: %w", err)
		}

		for _, arch := range archs {
			repo, tag, err := component.resolveImageRepoAndTag(ctx, image, arch)
			var tmp *DockerManifestNotFoundError
			if errors.As(err, &tmp) {
				logrus.Warnf("skipping image %s (%s) as no manifest found: %v", image, arch, err)
				continue
			} else if err != nil {
				return nil, fmt.Errorf("resolve image and tag for %s (%s): %w", image, arch, err)
			}
			newimage.Repo = repo
			newimage.Tag[arch] = tag
		}
		nextImages[component.name] = newimage
	}

	return nextImages, nil
}

func ComponentImageName(componentName, packageName, packageVersion string) (string, error) {
	registryServer := os.Getenv("IMAGES_REGISTRY_SERVER")
	if registryServer == "" {
		return "", fmt.Errorf("IMAGES_REGISTRY_SERVER not set")
	}
	tag, err := ComponentImageTag(componentName, packageName, packageVersion)
	if err != nil {
		return "", fmt.Errorf("component image tag: %w", err)
	}
	return fmt.Sprintf("%s/replicated/ec-%s:%s", registryServer, componentName, tag), nil
}

func ComponentImageTag(componentName, packageName, packageVersion string) (string, error) {
	if packageName == "" {
		return packageVersion, nil
	}
	packageVersion, err := ResolveApkoPackageVersion(componentName, packageName, packageVersion)
	if err != nil {
		return "", fmt.Errorf("apko output tag: %w", err)
	}
	return packageVersion, nil
}

// ResolveApkoPackageVersion resolves the fuzzy version matching in the apko config file to a specific version.
func ResolveApkoPackageVersion(componentName, packageName, packageVersion string) (string, error) {
	args := []string{
		"--silent",
		"apko-print-pkg-version",
		fmt.Sprintf("APKO_CONFIG=%s", filepath.Join("deploy", "images", componentName, "apko.tmpl.yaml")),
		fmt.Sprintf("PACKAGE_NAME=%s", packageName),
		fmt.Sprintf("PACKAGE_VERSION=%s", packageVersion),
	}
	var errBuf bytes.Buffer
	cmd := exec.Command("make", args...)
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("run command: %w: %s", err, errBuf.String())
	}
	return strings.TrimSpace(string(out)), nil
}

func GetImageNameFromBuildFile(imageBuildFile string) (string, error) {
	contents, err := os.ReadFile(imageBuildFile)
	if err != nil {
		return "", fmt.Errorf("read build file: %w", err)
	}
	if len(contents) == 0 {
		return "", fmt.Errorf("empty build/image file")
	}
	return strings.TrimSpace(string(contents)), nil
}

func FamiliarImageName(imageName string) string {
	ref, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		panic(fmt.Errorf("parse image name %s: %w", imageName, err))
	}
	return reference.FamiliarName(ref)
}

func latestPatchConstraint(s *semver.Version) string {
	return fmt.Sprintf(">=%d.%d,<%d.%d", s.Major(), s.Minor(), s.Major(), s.Minor()+1)
}

type filterFn func(string) bool

func GetGitHubRelease(ctx context.Context, owner, repo string, filter filterFn) (string, error) {
	client := github.NewClient(nil)
	if token := os.Getenv("GH_TOKEN"); token != "" {
		client = client.WithAuthToken(token)
	}
	releases, _, err := client.Repositories.ListReleases(
		ctx, owner, repo, &github.ListOptions{},
	)
	if err != nil {
		return "", err
	}
	for _, release := range releases {
		if !filter(release.GetTagName()) {
			continue
		}
		return release.GetTagName(), nil
	}
	return "", fmt.Errorf("filter returned no record")
}

// GetLatestGitHubTag returns the latest tag from a GitHub repository.
func GetLatestGitHubTag(ctx context.Context, owner, repo string) (string, error) {
	client := github.NewClient(nil)
	if token := os.Getenv("GH_TOKEN"); token != "" {
		client = client.WithAuthToken(token)
	}
	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, &github.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list tags: %w", err)
	}
	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found")
	}
	return tags[0].GetName(), nil
}

// GetLatestKotsHelmTag returns the correct tag from the kots-helm repository.
// this is not quite the same as the latest tag from the kots-helm repository, as github
// will list "v1.124.12" as being newer than "v1.124.12-build.0" and it is not in our usage.
func GetLatestKotsHelmTag(ctx context.Context) (string, error) {
	client := github.NewClient(nil)
	if token := os.Getenv("GH_TOKEN"); token != "" {
		client = client.WithAuthToken(token)
	}
	tags, _, err := client.Repositories.ListTags(ctx, "replicatedhq", "kots-helm", &github.ListOptions{PerPage: 100})
	if err != nil {
		return "", fmt.Errorf("list tags: %w", err)
	}
	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found")
	}
	latestTag := tags[0].GetName()
	logrus.Infof("latest tag: %s", latestTag)

	// check to see if there is a 'build.x' tag - if so, return that
	for _, tag := range tags {
		logrus.Infof("checkingtag: %s", tag.GetName())
		if !strings.HasPrefix(tag.GetName(), latestTag) {
			// tags are sorted, so once we find a tag that doesn't have the same prefix, we can break
			logrus.Infof("tag does not have same prefix: %s", tag.GetName())
			break
		}
		if strings.Contains(tag.GetName(), "-ec.") {
			logrus.Infof("tag is a ec tag, returning: %s", tag.GetName())
			return tag.GetName(), nil
		}
		if strings.Contains(tag.GetName(), "-build.") {
			logrus.Infof("tag is a build tag, returning: %s", tag.GetName())
			return tag.GetName(), nil
		}
	}
	return latestTag, nil
}

// GetGreatestGitHubTag returns the greatest non-prerelease semver tag from a GitHub repository
// that matches the provided constraints.
func GetGreatestGitHubTag(ctx context.Context, owner, repo string, constraints *semver.Constraints) (string, error) {
	client := github.NewClient(nil)
	if token := os.Getenv("GH_TOKEN"); token != "" {
		client = client.WithAuthToken(token)
	}
	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, &github.ListOptions{PerPage: 100})
	if err != nil {
		return "", fmt.Errorf("list tags: %w", err)
	}
	var best *semver.Version
	var bestStr string
	for _, tag := range tags {
		ver := tag.GetName()
		ver = strings.TrimPrefix(ver, "v")
		sv, err := semver.NewVersion(ver)
		if err != nil {
			continue
		}
		if sv.Prerelease() != "" {
			continue
		}
		if constraints != nil && !constraints.Check(sv) {
			continue
		}
		if best == nil || sv.GreaterThan(best) {
			best = sv
			bestStr = tag.GetName()
		}
	}
	if best == nil {
		return "", fmt.Errorf("no tags found matching constraints")
	}
	return bestStr, nil
}

func GetGreatestTagFromRegistry(ctx context.Context, ref string, constraints *semver.Constraints) (string, error) {
	repo, err := remote.NewRepository(ref)
	if err != nil {
		return "", fmt.Errorf("new repository: %w", err)
	}

	var best *semver.Version
	var bestStr string
	err = repo.Tags(ctx, "", func(tags []string) error {
		for _, tag := range tags {
			ver := tag
			ver = strings.TrimPrefix(ver, "v")
			sv, err := semver.NewVersion(ver)
			if err != nil {
				continue
			}
			if sv.Prerelease() != "" {
				continue
			}
			if constraints != nil && !constraints.Check(sv) {
				continue
			}
			if best != nil && sv.Equal(best) {
				// When versions are equal, prefer the more specific tag (e.g., 4.3.0 over 4.3)
				// Compare the original tag strings to determine which is more specific
				if len(tag) > len(bestStr) {
					best = sv
					bestStr = tag
				}
				continue
			}
			if best == nil || sv.GreaterThan(best) {
				best = sv
				bestStr = tag
				continue
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("list tags: %w", err)
	}
	if best == nil {
		return "", fmt.Errorf("no tags found matching constraints")
	}

	return bestStr, nil
}

func LatestChartVersion(ctx context.Context, hcli helm.Client, repo *repo.Entry, name string) (string, error) {
	logrus.Infof("adding helm repo %s", repo.Name)
	err := hcli.AddRepo(ctx, repo)
	if err != nil {
		return "", fmt.Errorf("add helm repo: %w", err)
	}
	logrus.Infof("finding latest chart version of %s/%s", repo, name)
	return hcli.Latest(ctx, repo.Name, name)
}

type DockerManifestNotFoundError struct {
	image, arch string
	err         error
}

func (e *DockerManifestNotFoundError) Error() string {
	return fmt.Sprintf("docker manifest not found for image %s and arch %s: %v", e.image, e.arch, e.err)
}

func GetImageDigest(ctx context.Context, img string, arch string) (string, error) {
	img, err := NormalizeDigestAndTag(img)
	if err != nil {
		return "", fmt.Errorf("normalize digest and tag: %w", err)
	}
	ref, err := docker.ParseReference("//" + img)
	if err != nil {
		return "", fmt.Errorf("parse reference: %w", err)
	}
	sysctx := &types.SystemContext{
		OSChoice:           "linux",
		ArchitectureChoice: arch,
	}
	src, err := ref.NewImageSource(ctx, sysctx)
	if err != nil {
		return "", fmt.Errorf("create image source: %w", err)
	}
	defer src.Close()

	manifraw, maniftype, err := src.GetManifest(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("get manifest: %w", err)
	}

	if !manifest.MIMETypeIsMultiImage(maniftype) {
		i, err := image.FromSource(ctx, sysctx, src)
		if err != nil {
			return "", fmt.Errorf("image from source: %w", err)
		}
		defer i.Close()
		info, err := i.Inspect(ctx)
		if err != nil {
			return "", fmt.Errorf("inspect image: %w", err)
		}
		if info.Architecture != arch {
			return "", &DockerManifestNotFoundError{image: img, arch: arch, err: err}
		}
		digest, err := manifest.Digest(manifraw)
		if err != nil {
			return "", fmt.Errorf("get manifest digest: %w", err)
		}
		return digest.String(), nil
	}

	manifestList, err := manifest.ListFromBlob(manifraw, maniftype)
	if err != nil {
		return "", fmt.Errorf("parse manifest list: %w", err)
	}

	// find the matching manifest for the linux/amd64 architecture
	for _, descriptor := range manifestList.Instances() {
		manifest, err := manifestList.Instance(descriptor)
		if err != nil {
			return "", fmt.Errorf("get manifest instance: %w", err)
		}
		if manifest.ReadOnly.Platform.Architecture != arch {
			continue
		}
		if manifest.ReadOnly.Platform.OS != "linux" {
			continue
		}
		return string(descriptor), nil
	}
	return "", &DockerManifestNotFoundError{image: img, arch: arch, err: err}
}

// NormalizeDigestAndTag returns the image name with the digest only if it has both a digest and a tag.
func NormalizeDigestAndTag(img string) (string, error) {
	ref, err := reference.ParseNormalizedNamed(img)
	if err != nil {
		return "", fmt.Errorf("parse normalized named: %w", err)
	}
	digested, ok := ref.(reference.Digested)
	if !ok {
		return img, nil
	}
	return fmt.Sprintf("%s@%s", ref.Name(), digested.Digest()), nil
}

// XXX we need to revisit this as a registry may have a port number.
func TagFromImage(image string) string {
	_, tag, _ := strings.Cut(image, ":")
	tag, _, _ = strings.Cut(tag, "@")
	return tag
}

// XXX we need to revisit this as a registry may have a port number.
func RemoveTagFromImage(image string) string {
	location, _, _ := strings.Cut(image, ":")
	return location
}

func MirrorChart(ctx context.Context, hcli helm.Client, repo *repo.Entry, name, ver string) error {
	logrus.Infof("adding helm repo %s", repo.Name)
	err := hcli.AddRepo(ctx, repo)
	if err != nil {
		return fmt.Errorf("add helm repo: %w", err)
	}

	logrus.Infof("pulling %s chart version %s", name, ver)
	chpath, err := hcli.Pull(ctx, repo.Name, name, ver)
	if err != nil {
		return fmt.Errorf("pull chart %s: %w", name, err)
	}
	logrus.Infof("downloaded %s chart: %s", name, chpath)
	defer os.Remove(chpath)

	srcMeta, err := hcli.GetChartMetadata(ctx, chpath, ver)
	if err != nil {
		return fmt.Errorf("get source chart metadata: %w", err)
	}

	if val := os.Getenv("CHARTS_REGISTRY_SERVER"); val != "" {
		logrus.Infof("authenticating with %q", os.Getenv("CHARTS_REGISTRY_SERVER"))
		if err := hcli.RegistryAuth(ctx,
			os.Getenv("CHARTS_REGISTRY_SERVER"),
			os.Getenv("CHARTS_REGISTRY_USER"),
			os.Getenv("CHARTS_REGISTRY_PASS"),
		); err != nil {
			return fmt.Errorf("registry authenticate: %w", err)
		}
	}

	dst := fmt.Sprintf("oci://%s", os.Getenv("CHARTS_DESTINATION"))
	chartURL := fmt.Sprintf("%s/%s", dst, name)
	logrus.Infof("verifying if destination tag already exists")
	dstMeta, err := hcli.GetChartMetadata(ctx, chartURL, ver)
	if err != nil && !strings.HasSuffix(err.Error(), "not found") {
		return fmt.Errorf("verify tag exists: %w", err)
	} else if err == nil {
		if srcMeta.AppVersion == dstMeta.AppVersion {
			logrus.Infof("cowardly refusing to override dst (tag %s already exist)", ver)
			return nil
		}
		logrus.Warnf("dst tag exists but app versions do not match (src: %s, dst: %s)", srcMeta.AppVersion, dstMeta.AppVersion)
		return nil
	}
	logrus.Infof("destination tag does not exist")

	logrus.Infof("pushing %s chart to %s", name, dst)
	if err := hcli.Push(ctx, chpath, dst); err != nil {
		return fmt.Errorf("push %s chart: %w", name, err)
	}
	remote := fmt.Sprintf("%s/%s:%s", dst, name, ver)
	logrus.Infof("pushed %s/%s chart: %s", repo, name, remote)
	return nil
}

func RunCommand(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func NewHelm() (helm.Client, error) {
	sv, err := getK0sVersion()
	if err != nil {
		return nil, fmt.Errorf("get k0s version: %w", err)
	}
	return helm.NewClient(helm.HelmOptions{
		HelmPath:              "helm",        // use the helm binary in PATH
		KubernetesEnvSettings: helmcli.New(), // use the default env settings from helm
		K8sVersion:            sv.Original(),
		Writer:                logrus.New().Writer(),
	})
}

func GetLatestKubernetesVersion() (*semver.Version, error) {
	resp, err := http.Get("https://dl.k8s.io/release/stable.txt")
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		return nil, fmt.Errorf("no content in stable.txt")
	}
	return semver.NewVersion(scanner.Text())
}

func GetSupportedArchs() []string {
	return []string{"amd64", "arm64"}
}

func getLatestImageNameAndTag(ctx context.Context, ref string, constraints *semver.Constraints) (string, error) {
	tag, err := GetGreatestTagFromRegistry(ctx, ref, constraints)
	if err != nil {
		return "", fmt.Errorf("get greatest tag from ref %s: %w", ref, err)
	}
	return fmt.Sprintf("%s:%s", ref, tag), nil
}

func addProxyAnonymousPrefix(image string) string {
	if strings.HasPrefix(image, "proxy.replicated.com/") {
		return image
	}
	return fmt.Sprintf("proxy.replicated.com/anonymous/%s", image)
}
