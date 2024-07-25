package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/google/go-github/v62/github"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"k8s.io/utils/strings/slices"
)

const (
	wolfiAPKIndexURL = "https://packages.wolfi.dev/os/x86_64/APKINDEX.tar.gz"
)

func GetWolfiAPKIndex() ([]byte, error) {
	tmpdir, err := os.MkdirTemp("", "wolfi-apk-index")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpdir)
	if err := DownloadFile(wolfiAPKIndexURL, filepath.Join(tmpdir, "APKINDEX.tar.gz")); err != nil {
		return nil, fmt.Errorf("download APKINDEX.tar.gz: %w", err)
	}
	if err := ExtractTGZArchive(filepath.Join(tmpdir, "APKINDEX.tar.gz"), tmpdir); err != nil {
		return nil, fmt.Errorf("extract APKINDEX.tar.gz: %w", err)
	}
	contents, err := os.ReadFile(filepath.Join(tmpdir, "APKINDEX"))
	if err != nil {
		return nil, fmt.Errorf("read APKINDEX: %w", err)
	}
	return contents, nil
}

type PackageVersion struct {
	semver   semver.Version
	revision int
}

func (v *PackageVersion) String() string {
	return fmt.Sprintf("%s-r%d", v.semver.Original(), v.revision)
}

type PackageVersions []*PackageVersion

func (pvs PackageVersions) Len() int {
	return len(pvs)
}

func (pvs PackageVersions) Less(i, j int) bool {
	if pvs[i].semver.Equal(&pvs[j].semver) {
		return pvs[i].revision < pvs[j].revision
	}
	return pvs[i].semver.LessThan(&pvs[j].semver)
}

func (pvs PackageVersions) Swap(i, j int) {
	pvs[i], pvs[j] = pvs[j], pvs[i]
}

func ParsePackageVersion(version string) (*PackageVersion, error) {
	parts := strings.Split(version, "-r")
	if len(parts) != 2 {
		return nil, fmt.Errorf("incorrect number of parts in version %s", version)
	}
	sv, err := semver.NewVersion(parts[0])
	if err != nil {
		return nil, fmt.Errorf("parse version: %w", err)
	}
	revision, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("parse revision: %w", err)
	}
	return &PackageVersion{semver: *sv, revision: revision}, nil
}

func MustParsePackageVersion(version string) *PackageVersion {
	pv, err := ParsePackageVersion(version)
	if err != nil {
		panic(err)
	}
	return pv
}

// listWolfiPackageVersions returns a list of all versions for a given package name
func listWolfiPackageVersions(wolfiAPKIndex []byte, pkgName string) ([]*PackageVersion, error) {
	var versions []*PackageVersion
	scanner := bufio.NewScanner(bytes.NewReader(wolfiAPKIndex))
	for scanner.Scan() {
		line := scanner.Text()
		// filter by package name
		if line != "P:"+pkgName {
			continue
		}
		scanner.Scan()
		line = scanner.Text()
		if !strings.HasPrefix(line, "V:") {
			return nil, fmt.Errorf("incorrect APKINDEX version line: %s", line)
		}
		// extract the version
		pv, err := ParsePackageVersion(line[2:])
		if err != nil {
			return nil, fmt.Errorf("parse package version from line %s: %w", line, err)
		}
		versions = append(versions, pv)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan APKINDEX: %w", err)
	}
	return versions, nil
}

// listMatchingWolfiPackageVersions returns a list of all versions for a given package name that
// match the semver constraints.
func listMatchingWolfiPackageVersions(wolfiAPKIndex []byte, pkgName string, constraints *semver.Constraints) ([]*PackageVersion, error) {
	versions, err := listWolfiPackageVersions(wolfiAPKIndex, pkgName)
	if err != nil {
		return nil, fmt.Errorf("list package versions: %w", err)
	}

	if constraints == nil {
		return versions, nil
	}

	var matchingVersions []*PackageVersion
	for _, version := range versions {
		if !constraints.Check(&version.semver) {
			continue
		}
		matchingVersions = append(matchingVersions, version)
	}
	return matchingVersions, nil
}

// FindWolfiPackageVersion returns the latest version and revision of a package in the wolfi APK
// index that matches the semver constraints.
func FindWolfiPackageVersion(wolfiAPKIndex []byte, pkgName string, constraints *semver.Constraints) (string, error) {
	versions, err := listMatchingWolfiPackageVersions(wolfiAPKIndex, pkgName, constraints)
	if err != nil {
		return "", fmt.Errorf("list package versions: %w", err)
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("package %q not found with the provided constraints", pkgName)
	}

	sorted := PackageVersions(versions)
	sort.Sort(sorted)

	return sorted[len(sorted)-1].String(), nil
}

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

func ApkoBuildAndPublish(componentName string, packageName string, packageVersion string, upstreamVersion string) error {
	args := []string{
		"apko-build-and-publish",
		fmt.Sprintf("IMAGE=%s/replicated/ec-%s:%s", os.Getenv("IMAGES_REGISTRY_SERVER"), componentName, packageVersion),
		fmt.Sprintf("APKO_CONFIG=%s", filepath.Join("deploy", "images", componentName, "apko.tmpl.yaml")),
		fmt.Sprintf("PACKAGE_NAME=%s", packageName),
		fmt.Sprintf("PACKAGE_VERSION=%s", packageVersion),
		fmt.Sprintf("UPSTREAM_VERSION=%s", upstreamVersion),
	}
	cmd := exec.Command("make", args...)
	if err := RunCommand(cmd); err != nil {
		return err
	}
	return nil
}

func GetDigestFromBuildFile() (string, error) {
	contents, err := os.ReadFile("build/digest")
	if err != nil {
		return "", fmt.Errorf("read build file: %w", err)
	}
	parts := strings.Split(string(contents), "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("incorrect number of parts in build file")
	}
	return strings.TrimSpace(parts[1]), nil
}

func GetLatestGitHubRelease(ctx context.Context, owner, repo string) (string, error) {
	client := github.NewClient(nil)
	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return "", err
	}
	return release.GetName(), nil
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
	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, &github.ListOptions{})
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

func LatestChartVersion(repo, name string) (string, error) {
	hcli, err := NewHelm()
	if err != nil {
		return "", fmt.Errorf("create helm client: %w", err)
	}
	defer hcli.Close()
	return hcli.Latest(repo, name)
}

func GetImageDigest(ctx context.Context, image string) (string, error) {
	ref, err := docker.ParseReference("//" + image)
	if err != nil {
		return "", fmt.Errorf("parse image reference: %w", err)
	}
	sysctx := &types.SystemContext{}
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
		digest, err := docker.GetDigest(ctx, nil, ref)
		return string(digest), err
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
		if manifest.ReadOnly.Platform.Architecture != "amd64" {
			continue
		}
		if manifest.ReadOnly.Platform.OS != "linux" {
			continue
		}
		return string(descriptor), nil
	}
	return "", fmt.Errorf("failed to locate linux/amd64 manifest")
}

// easier than walking through map[interface{}]interface{}
type ReducedContainer struct {
	Image string `yaml:"image"`
}

type ReducedNestedSpec struct {
	Containers     []ReducedContainer `yaml:"containers"`
	InitContainers []ReducedContainer `yaml:"initContainers"`
}

type ReducedTemplate struct {
	Spec ReducedNestedSpec `yaml:"spec"`
}

type ReducedSpec struct {
	Template       ReducedTemplate    `yaml:"template"`
	Containers     []ReducedContainer `yaml:"containers"`
	InitContainers []ReducedContainer `yaml:"initContainers"`
}

type ReducedResource struct {
	Kind string      `yaml:"kind"`
	Spec ReducedSpec `yaml:"spec"`
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

func RenderChartAndFindImageDigest(ctx context.Context, repo, name, version string, values map[string]interface{}, image string) (string, error) {
	logrus.Infof("getting a list of images from chart %s/%s (%s)", repo, name, version)
	images, err := GetImagesFromChart(repo, name, version, values)
	if err != nil {
		return "", fmt.Errorf("get images from %s/%s chart: %w", repo, name, err)
	}

	desired := []string{}
	logrus.Infof("searching for %s image", image)
	desired = slices.Filter(desired, images, func(a string) bool {
		i := strings.Split(a, ":")
		return i[0] == image
	})
	if len(desired) != 1 {
		return "", fmt.Errorf("found %d images for %s, expected 1", len(desired), image)
	}
	logrus.Infof("found %s image: %s", image, desired[0])

	logrus.Infof("finding the digest for %s", desired[0])
	digest, err := GetImageDigest(ctx, desired[0])
	if err != nil {
		return "", fmt.Errorf("get digest for %s: %w", desired[0], err)
	}

	_, tag, _ := strings.Cut(desired[0], ":")
	imgver := fmt.Sprintf("%s@%s", tag, digest)
	logrus.Infof("found %s image: %s", image, imgver)
	return imgver, nil
}

func GetImagesFromOCIChart(url, name, version string, values map[string]interface{}) ([]string, error) {
	hcli, err := NewHelm()
	if err != nil {
		return nil, fmt.Errorf("create helm client: %w", err)
	}
	defer hcli.Close()

	chartPath, err := hcli.PullOCI(url, version)
	if err != nil {
		return nil, err
	}

	return GetImagesFromLocalChart(name, chartPath, values)
}

func GetImagesFromChart(repo, name, version string, values map[string]interface{}) ([]string, error) {
	hcli, err := NewHelm()
	if err != nil {
		return nil, fmt.Errorf("create helm client: %w", err)
	}
	defer hcli.Close()

	chartPath, err := hcli.Pull(repo, name, version)
	if err != nil {
		return nil, err
	}

	return GetImagesFromLocalChart(name, chartPath, values)
}

func GetImagesFromLocalChart(name, path string, values map[string]interface{}) ([]string, error) {
	hcli, err := NewHelm()
	if err != nil {
		return nil, fmt.Errorf("create helm client: %w", err)
	}
	defer hcli.Close()

	chartResources, err := hcli.Render(name, path, values, "default")
	if err != nil {
		return nil, err
	}

	images := []string{}
	for _, resource := range chartResources {
		r := ReducedResource{}
		if err := yaml.Unmarshal([]byte(resource), &r); err != nil {
			return nil, err
		}

		for _, container := range r.Spec.Containers {
			if !slices.Contains(images, container.Image) {
				images = append(images, container.Image)
			}
		}

		for _, container := range r.Spec.Template.Spec.Containers {
			if !slices.Contains(images, container.Image) {
				images = append(images, container.Image)
			}
		}

		for _, container := range r.Spec.InitContainers {
			if !slices.Contains(images, container.Image) {
				images = append(images, container.Image)
			}
		}

		for _, container := range r.Spec.Template.Spec.InitContainers {
			if !slices.Contains(images, container.Image) {
				images = append(images, container.Image)
			}
		}
	}

	for i, image := range images {
		// Normalize the image name to include docker.io and tag
		ref, err := docker.ParseReference("//" + image)
		if err != nil {
			return nil, fmt.Errorf("parse image reference %s: %w", image, err)
		}
		images[i] = ref.DockerReference().String()
	}

	return images, nil
}

func MirrorChart(repo, name, ver string) error {
	hcli, err := NewHelm()
	if err != nil {
		return fmt.Errorf("create helm client: %w", err)
	}
	defer hcli.Close()

	logrus.Infof("pulling %s chart version %s", name, ver)
	chpath, err := hcli.Pull(repo, name, ver)
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

func DownloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http get %s: %w", url, err)
	}
	defer resp.Body.Close()

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file %s: %w", dest, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func ExtractTGZArchive(tgzFile string, destDir string) error {
	fileReader, err := os.Open(tgzFile)
	if err != nil {
		return fmt.Errorf("open tgz file %q: %w", tgzFile, err)
	}
	defer fileReader.Close()

	gzReader, err := gzip.NewReader(fileReader)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar data: %w", err)
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		err = func() error {
			fileName := filepath.Join(destDir, hdr.Name)

			parentDir := filepath.Dir(fileName)
			err := os.MkdirAll(parentDir, 0755)
			if err != nil {
				return fmt.Errorf("create directory %q: %w", parentDir, err)
			}

			fileWriter, err := os.Create(fileName)
			if err != nil {
				return fmt.Errorf("create file %q: %w", hdr.Name, err)
			}
			defer fileWriter.Close()

			_, err = io.Copy(fileWriter, tarReader)
			if err != nil {
				return fmt.Errorf("write file %q: %w", hdr.Name, err)
			}

			return nil
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

func RunCommand(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
