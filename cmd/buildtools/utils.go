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

func GetWolfiPackageVersion(wolfiAPKIndex []byte, pkgName string, pinnedVersion string) (string, error) {
	// example APKINDEX content:
	// P:calico-node
	// V:3.26.1-r1
	// ...
	//
	// P:calico-node
	// V:3.26.1-r10
	// ...
	//
	// P:calico-node
	// V:3.26.1-r9
	// ...

	var revisions []int
	scanner := bufio.NewScanner(bytes.NewReader(wolfiAPKIndex))
	for scanner.Scan() {
		line := scanner.Text()
		// filter by package name
		if line != "P:"+pkgName {
			continue
		}
		scanner.Scan()
		line = scanner.Text()
		// filter by pinned version
		if pinnedVersion != "" && !strings.HasPrefix(line, "V:"+pinnedVersion+"-r") {
			continue
		}
		// find the revision number
		parts := strings.Split(line, "-r")
		if len(parts) != 2 {
			return "", fmt.Errorf("incorrect number of parts in APKINDEX line: %s", line)
		}
		revision, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", fmt.Errorf("parse revision: %w", err)
		}
		revisions = append(revisions, revision)
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan APKINDEX: %w", err)
	}

	if len(revisions) == 0 {
		return "", fmt.Errorf("package %q not found", pkgName)
	}

	// get the latest revision

	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i] > revisions[j]
	})

	return fmt.Sprintf("%s-r%d", pinnedVersion, revisions[0]), nil
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
			return fmt.Errorf("apko login: %w", err)
		}
	}
	return nil
}

func ApkoBuildAndPublish(componentName string, packageVersion string) error {
	if err := RunCommand(
		"make",
		"apko-build-and-publish",
		fmt.Sprintf("IMAGE=%s/replicated/ec-%s:%s", os.Getenv("REGISTRY_SERVER"), componentName, packageVersion),
		fmt.Sprintf("APKO_CONFIG=%s", filepath.Join("deploy", "images", componentName, "apko.tmpl.yaml")),
		fmt.Sprintf("PACKAGE_VERSION=%s", packageVersion),
	); err != nil {
		return fmt.Errorf("failed to build and publish apko for %s: %w", componentName, err)
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
