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
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v62/github"
	"github.com/sirupsen/logrus"
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

func (v *PackageVersion) matches(version *semver.Version) bool {
	parts := strings.SplitN(version.Original(), "-", 2)
	parts = strings.SplitN(parts[0], "+", 2)
	parts = strings.Split(parts[0], ".")
	switch len(parts) {
	case 1:
		return v.semver.Major() == version.Major()
	case 2:
		return v.semver.Major() == version.Major() && v.semver.Minor() == version.Minor()
	default:
		return v.semver.Major() == version.Major() && v.semver.Minor() == version.Minor() && v.semver.Patch() == version.Patch()
	}
}

func (v *PackageVersion) String() string {
	return fmt.Sprintf("%s-r%d", v.semver.Original(), v.revision)
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
// match the pinned version based on the number of version segments.
func listMatchingWolfiPackageVersions(wolfiAPKIndex []byte, pkgName, pinnedVersion string) ([]*PackageVersion, error) {
	pinnedSV, err := semver.NewVersion(pinnedVersion)
	if err != nil {
		return nil, fmt.Errorf("parse pinned version: %w", err)
	}

	versions, err := listWolfiPackageVersions(wolfiAPKIndex, pkgName)
	if err != nil {
		return nil, fmt.Errorf("list package versions: %w", err)
	}

	if pinnedVersion == "" {
		return versions, nil
	}

	var matchingVersions []*PackageVersion
	for _, version := range versions {
		// filter by package version
		if !version.matches(pinnedSV) {
			continue
		}
		matchingVersions = append(matchingVersions, version)
	}
	return matchingVersions, nil
}

// GetWolfiPackageVersion returns the latest version and revision of a package in the wolfi APK
// index that matches the pinned version based on the number of version segments.
func GetWolfiPackageVersion(wolfiAPKIndex []byte, pkgName, pinnedVersion string) (string, error) {
	versions, err := listMatchingWolfiPackageVersions(wolfiAPKIndex, pkgName, pinnedVersion)
	if err != nil {
		return "", fmt.Errorf("list package versions: %w", err)
	}

	var maxVersion *PackageVersion
	for _, version := range versions {
		if maxVersion == nil {
			maxVersion = version
		} else if version.semver.GreaterThan(&maxVersion.semver) {
			maxVersion = version
		} else if version.semver.Equal(&maxVersion.semver) && version.revision > maxVersion.revision {
			maxVersion = version
		}
	}

	if maxVersion == nil {
		return "", fmt.Errorf("package %q not found", pkgName)
	}

	return maxVersion.String(), nil
}

func ApkoLogin() error {
	if err := RunCommand("make", "apko"); err != nil {
		return fmt.Errorf("make apko: %w", err)
	}
	if os.Getenv("REGISTRY_PASS") != "" {
		if err := RunCommand(
			"make",
			"apko-login",
			fmt.Sprintf("REGISTRY=%s", os.Getenv("REGISTRY_SERVER")),
			fmt.Sprintf("USERNAME=%s", os.Getenv("REGISTRY_USER")),
			fmt.Sprintf("PASSWORD=%s", os.Getenv("REGISTRY_PASS")),
		); err != nil {
			return err
		}
	}
	return nil
}

func ApkoBuildAndPublish(componentName string, packageVersion string, extraArgs ...string) error {
	args := []string{
		"apko-build-and-publish",
		fmt.Sprintf("IMAGE=%s/replicated/ec-%s:%s", os.Getenv("REGISTRY_SERVER"), componentName, packageVersion),
		fmt.Sprintf("APKO_CONFIG=%s", filepath.Join("deploy", "images", componentName, "apko.tmpl.yaml")),
		fmt.Sprintf("PACKAGE_VERSION=%s", packageVersion),
	}
	args = append(args, extraArgs...)
	if err := RunCommand("make", args...); err != nil {
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

func GetLatestGitHubTag(ctx context.Context, owner, repo string) (string, error) {
	client := github.NewClient(nil)
	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, &github.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to list tags: %w", err)
	}
	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found")
	}
	for _, tag := range tags {
		// "-" indicates this is a pre-release version
		if strings.Contains(tag.GetName(), "-") {
			continue
		}
		return tag.GetName(), nil
	}
	return "", fmt.Errorf("no stable tags found")
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
		return fmt.Errorf("unable to open ./Makefile: %w", err)
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
		return fmt.Errorf("unable to open ./Makefile: %w", err)
	}
	defer wfile.Close()

	for _, line := range lines {
		if _, err := fmt.Fprintln(wfile, line); err != nil {
			return fmt.Errorf("unable to write ./Makefile: %w", err)
		}
	}
	return nil
}

func LatestChartVersion(repo, name string) (string, error) {
	hcli, err := NewHelm()
	if err != nil {
		return "", fmt.Errorf("unable to create helm client: %v", err)
	}
	defer hcli.Close()
	return hcli.Latest(repo, name)
}

func MirrorChart(repo, name, ver string) error {
	hcli, err := NewHelm()
	if err != nil {
		return fmt.Errorf("unable to create helm: %w", err)
	}
	defer hcli.Close()

	logrus.Infof("pulling %s chart version %s", name, ver)
	chpath, err := hcli.Pull(repo, name, ver)
	if err != nil {
		return fmt.Errorf("unable to pull %s: %w", name, err)
	}
	logrus.Infof("downloaded %s chart: %s", name, chpath)
	defer os.Remove(chpath)

	if val := os.Getenv("REGISTRY_SERVER"); val != "" {
		logrus.Infof("authenticating with %q", os.Getenv("REGISTRY_SERVER"))
		if err := hcli.RegistryAuth(
			os.Getenv("REGISTRY_SERVER"),
			os.Getenv("REGISTRY_USER"),
			os.Getenv("REGISTRY_PASS"),
		); err != nil {
			return fmt.Errorf("unable to authenticate: %w", err)
		}
	}

	dst := os.Getenv("DESTINATION")
	logrus.Infof("verifying if destination tag already exists")
	tmpf, err := hcli.Pull(dst, name, ver)
	if err != nil && !strings.HasSuffix(err.Error(), "not found") {
		return fmt.Errorf("unable to verify if tag already exists: %w", err)
	} else if err == nil {
		os.Remove(tmpf)
		logrus.Warnf("cowardly refusing to override dst (tag %s already exist)", ver)
		return nil
	}
	logrus.Infof("destination tag does not exist")

	logrus.Infof("pushing %s chart to %s", name, dst)
	if err := hcli.Push(chpath, dst); err != nil {
		return fmt.Errorf("unable to push openebs: %w", err)
	}
	remote := fmt.Sprintf("%s/%s:%s", dst, name, ver)
	logrus.Infof("pushed openebs chart: %s", remote)
	return nil
}

func DownloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("unable to get %s: %w", url, err)
	}
	defer resp.Body.Close()

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("unable to create %s: %w", dest, err)
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

func RunCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
