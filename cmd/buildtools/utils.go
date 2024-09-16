package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/distribution/reference"
	"github.com/google/go-github/v62/github"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/repo"
)

func ApkoLogin() error {
	cmd := exec.Command("make", "apko")
	if err := RunCommand(cmd); err != nil {
		return fmt.Errorf("make apko: %w", err)
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
			return err
		}
	}
	return nil
}

func ApkoBuildAndPublish(componentName, packageName, packageVersion string, arch string) error {
	image, err := ComponentImageName(componentName, packageName, packageVersion, arch)
	if err != nil {
		return fmt.Errorf("component image name: %w", err)
	}
	args := []string{
		"apko-build-and-publish",
		fmt.Sprintf("IMAGE=%s", image),
		fmt.Sprintf("APKO_CONFIG=%s", filepath.Join("deploy", "images", componentName, "apko.tmpl.yaml")),
		fmt.Sprintf("PACKAGE_VERSION=%s", packageVersion),
		fmt.Sprintf("ARCHS=%s", arch),
	}
	cmd := exec.Command("make", args...)
	if err := RunCommand(cmd); err != nil {
		return err
	}
	return nil
}

func ComponentImageName(componentName, packageName, packageVersion, arch string) (string, error) {
	registryServer := os.Getenv("IMAGES_REGISTRY_SERVER")
	if registryServer == "" {
		return "", fmt.Errorf("IMAGES_REGISTRY_SERVER not set")
	}
	tag, err := ComponentImageTag(componentName, packageName, packageVersion, arch)
	if err != nil {
		return "", fmt.Errorf("component image tag: %w", err)
	}
	return fmt.Sprintf("%s/replicated/ec-%s:%s", registryServer, componentName, tag), nil
}

func ComponentImageTag(componentName, packageName, packageVersion, arch string) (string, error) {
	if packageName == "" {
		return packageVersion, nil
	}
	packageVersion, err := ResolveApkoPackageVersion(componentName, packageName, packageVersion)
	if err != nil {
		return "", fmt.Errorf("apko output tag: %w", err)
	}
	tag := fmt.Sprintf("%s-%s", packageVersion, arch)
	return tag, nil
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
	cmd := exec.Command("make", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("run command: %w", err)
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

func GetLatestGitHubRelease(ctx context.Context, owner, repo string) (string, error) {
	client := github.NewClient(nil)
	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return "", err
	}
	return release.GetName(), nil
}

func latestPatchConstraint(s *semver.Version) string {
	return fmt.Sprintf(">=%d.%d,<%d.%d", s.Major(), s.Minor(), s.Major(), s.Minor()+1)
}

type filterFn func(string) bool

func GetGitHubRelease(ctx context.Context, owner, repo string, filter filterFn) (string, error) {
	client := github.NewClient(nil)
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
	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, &github.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list tags: %w", err)
	}
	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found")
	}
	return tags[0].GetName(), nil
}

// GetGreatestGitHubTag returns the greatest non-prerelease semver tag from a GitHub repository
// that matches the provided constraints.
func GetGreatestGitHubTag(ctx context.Context, owner, repo string, constrants *semver.Constraints) (string, error) {
	client := github.NewClient(nil)
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
		if !constrants.Check(sv) {
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

func GetMakefileVariable(name string) (string, error) {
	f, err := os.Open("./Makefile")
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		re := regexp.MustCompile(fmt.Sprintf("^%s ?= ?", regexp.QuoteMeta(name)))
		if !re.MatchString(line) {
			continue
		}
		slices := strings.Split(line, "=")
		if len(slices) != 2 {
			return "", nil
		}
		return strings.TrimSpace(slices[1]), nil
	}
	return "", fmt.Errorf("variable %s not found in ./Makefile", name)
}

func SetMakefileVariable(name, value string) error {
	file, err := os.OpenFile("./Makefile", os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open ./Makefile: %w", err)
	}
	defer file.Close()

	var found int
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		re := regexp.MustCompile(fmt.Sprintf("^%s ?= ?", regexp.QuoteMeta(name)))
		if !re.MatchString(text) {
			lines = append(lines, text)
			continue
		}
		line := fmt.Sprintf("%s = %s", name, value)
		lines = append(lines, line)
		found++
	}

	if found != 1 {
		if found == 0 {
			return fmt.Errorf("variable %s not found in ./Makefile", name)
		}
		return fmt.Errorf("variable %s found %d times in ./Makefile", name, found)
	}

	wfile, err := os.OpenFile("./Makefile", os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open ./Makefile: %w", err)
	}
	defer wfile.Close()

	for _, line := range lines {
		if _, err := fmt.Fprintln(wfile, line); err != nil {
			return fmt.Errorf("write ./Makefile: %w", err)
		}
	}
	return nil
}

func LatestChartVersion(repo *repo.Entry, name string) (string, error) {
	hcli, err := NewHelm()
	if err != nil {
		return "", fmt.Errorf("create helm client: %w", err)
	}
	defer hcli.Close()
	logrus.Infof("adding helm repo %s", repo.Name)
	err = hcli.AddRepo(repo)
	if err != nil {
		return "", fmt.Errorf("add helm repo: %w", err)
	}
	logrus.Infof("finding latest chart version of %s/%s", repo, name)
	return hcli.Latest(repo.Name, name)
}

type DockerManifestNotFoundError struct {
	image, arch string
	err         error
}

func (e *DockerManifestNotFoundError) Error() string {
	return fmt.Sprintf("docker manifest not found for image %s and arch %s: %v", e.image, e.arch, e.err)
}

func GetImageDigest(ctx context.Context, img string, arch string) (string, error) {
	ref, err := docker.ParseReference("//" + img)
	if err != nil {
		return "", fmt.Errorf("parse image reference: %w", err)
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

// XXX we need to revisit this as a registry may have a port number.
func TagFromImage(image string) string {
	_, tag, _ := strings.Cut(image, ":")
	return tag
}

// XXX we need to revisit this as a registry may have a port number.
func RemoveTagFromImage(image string) string {
	location, _, _ := strings.Cut(image, ":")
	return location
}

func GetImagesFromOCIChart(url, name, version string, values map[string]interface{}) ([]string, error) {
	hcli, err := NewHelm()
	if err != nil {
		return nil, fmt.Errorf("create helm client: %w", err)
	}
	defer hcli.Close()

	return helm.ExtractImagesFromOCIChart(hcli, url, name, version, values)
}

func MirrorChart(repo *repo.Entry, name, ver string) error {
	hcli, err := NewHelm()
	if err != nil {
		return fmt.Errorf("create helm client: %w", err)
	}
	defer hcli.Close()

	logrus.Infof("adding helm repo %s", repo.Name)
	err = hcli.AddRepo(repo)
	if err != nil {
		return fmt.Errorf("add helm repo: %w", err)
	}

	logrus.Infof("pulling %s chart version %s", name, ver)
	chpath, err := hcli.Pull(repo.Name, name, ver)
	if err != nil {
		return fmt.Errorf("pull chart %s: %w", name, err)
	}
	logrus.Infof("downloaded %s chart: %s", name, chpath)
	defer os.Remove(chpath)

	if val := os.Getenv("CHARTS_REGISTRY_SERVER"); val != "" {
		logrus.Infof("authenticating with %q", os.Getenv("CHARTS_REGISTRY_SERVER"))
		if err := hcli.RegistryAuth(
			os.Getenv("CHARTS_REGISTRY_SERVER"),
			os.Getenv("CHARTS_REGISTRY_USER"),
			os.Getenv("CHARTS_REGISTRY_PASS"),
		); err != nil {
			return fmt.Errorf("registry authenticate: %w", err)
		}
	}

	dst := fmt.Sprintf("oci://%s", os.Getenv("CHARTS_DESTINATION"))
	logrus.Infof("verifying if destination tag already exists")
	tmpf, err := hcli.Pull(dst, name, ver)
	if err != nil && !strings.HasSuffix(err.Error(), "not found") {
		return fmt.Errorf("verify tag exists: %w", err)
	} else if err == nil {
		os.Remove(tmpf)
		logrus.Warnf("cowardly refusing to override dst (tag %s already exist)", ver)
		return nil
	}
	logrus.Infof("destination tag does not exist")

	logrus.Infof("pushing %s chart to %s", name, dst)
	if err := hcli.Push(chpath, dst); err != nil {
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

func NewHelm() (*helm.Helm, error) {
	sv, err := getK0sVersion()
	if err != nil {
		return nil, fmt.Errorf("get k0s version: %w", err)
	}
	return helm.NewHelm(helm.HelmOptions{
		Writer:     logrus.New().Writer(),
		K0sVersion: sv.Original(),
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
