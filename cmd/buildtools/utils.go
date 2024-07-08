package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/google/go-github/v62/github"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"k8s.io/utils/strings/slices"
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
		return "", fmt.Errorf("unable to create helm client: %w", err)
	}
	defer hcli.Close()
	return hcli.Latest(repo, name)
}

func GetImageDigest(ctx context.Context, image string) (string, error) {
	ref, err := docker.ParseReference("//" + image)
	if err != nil {
		return "", fmt.Errorf("unable to parse image reference: %w", err)
	}
	sysctx := &types.SystemContext{}
	src, err := ref.NewImageSource(ctx, sysctx)
	if err != nil {
		return "", fmt.Errorf("unable to create image source: %w", err)
	}
	defer src.Close()

	manifraw, maniftype, err := src.GetManifest(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("error getting manifest: %w", err)
	}

	if !manifest.MIMETypeIsMultiImage(maniftype) {
		digest, err := docker.GetDigest(ctx, nil, ref)
		return string(digest), err
	}

	manifestList, err := manifest.ListFromBlob(manifraw, maniftype)
	if err != nil {
		return "", fmt.Errorf("error parsing manifest list: %w", err)
	}

	// find the matching manifest for the linux/amd64 architecture
	for _, descriptor := range manifestList.Instances() {
		manifest, err := manifestList.Instance(descriptor)
		if err != nil {
			return "", fmt.Errorf("error getting manifest instance: %w", err)
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

type RecudedNestedSpec struct {
	Containers     []ReducedContainer `yaml:"containers"`
	InitContainers []ReducedContainer `yaml:"initContainers"`
}

type ReducedTemplate struct {
	Spec RecudedNestedSpec `yaml:"spec"`
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

func RenderChartAndFindImageDigest(ctx context.Context, repo, name, version string, values map[string]interface{}, image string) (string, error) {
	logrus.Infof("getting a list of images from chart %s/%s (%s)", repo, name, version)
	images, err := GetImagesFromChart(repo, name, version, values)
	if err != nil {
		return "", fmt.Errorf("unable to get images from velero chart: %w", err)
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
		return "", fmt.Errorf("unable to get digest for %s: %w", desired[0], err)
	}

	_, tag, _ := strings.Cut(desired[0], ":")
	imgver := fmt.Sprintf("%s@%s", tag, digest)
	logrus.Infof("found %s image: %s", image, imgver)
	return imgver, nil
}

func GetImagesFromChart(repo, name, version string, values map[string]interface{}) ([]string, error) {
	hcli, err := NewHelm()
	if err != nil {
		return nil, fmt.Errorf("unable to create helm client: %w", err)
	}
	defer hcli.Close()

	chartPath, err := hcli.Pull(repo, name, version)
	if err != nil {
		return nil, err
	}

	chartResources, err := hcli.Render(name, chartPath, values, "default")
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

	return images, nil
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
	logrus.Infof("pushed %s/%s chart: %s", repo, name, remote)
	return nil
}
