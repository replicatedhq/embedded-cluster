package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/google/go-github/v62/github"
	"github.com/sirupsen/logrus"
)

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
