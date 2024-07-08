package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/pusher"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/uploader"
)

var (
	// getters is a list of known getters for both http and
	// oci schemes.
	getters = getter.Providers{
		getter.Provider{
			Schemes: []string{"http", "https"},
			New:     getter.NewHTTPGetter,
		},
		getter.Provider{
			Schemes: []string{"oci"},
			New:     getter.NewOCIGetter,
		},
	}

	// pushers holds all supported pushers (uploaders).
	pushers = pusher.Providers{
		pusher.Provider{
			Schemes: []string{"oci"},
			New:     pusher.NewOCIPusher,
		},
	}

	// repositories holds a list of all known repositories
	// we use to pull charts from.
	repositories = repo.File{
		Repositories: []*repo.Entry{
			{
				Name: "openebs",
				URL:  "https://openebs.github.io/openebs",
			},
			{
				Name: "seaweedfs",
				URL:  "https://seaweedfs.github.io/seaweedfs/helm",
			},
			{
				Name: "twuni",
				URL:  "https://helm.twun.io",
			},
			{
				Name: "vmware-tanzu",
				URL:  "https://vmware-tanzu.github.io/helm-charts",
			},
		},
	}
)

func NewHelm() (*Helm, error) {
	tmpdir, err := os.MkdirTemp(os.TempDir(), "helm-cache-*")
	if err != nil {
		return nil, err
	}
	writer := logrus.New().Writer()
	regcli, err := registry.NewClient(registry.ClientOptWriter(writer))
	if err != nil {
		return nil, fmt.Errorf("unable to create registry client: %w", err)
	}
	return &Helm{
		tmpdir: tmpdir,
		regcli: regcli,
	}, nil
}

type Helm struct {
	tmpdir  string
	repocfg string
	regcli  *registry.Client
}

func (h *Helm) prepare() error {
	if h.repocfg != "" {
		return nil
	}

	data, err := yaml.Marshal(repositories)
	if err != nil {
		return fmt.Errorf("unable to marshal repositories: %w", err)
	}

	repocfg := filepath.Join(h.tmpdir, "config.yaml")
	if err := os.WriteFile(repocfg, data, 0644); err != nil {
		return fmt.Errorf("unable to write repositories: %w", err)
	}

	for _, repository := range repositories.Repositories {
		chrepo, err := repo.NewChartRepository(
			repository, getters,
		)
		if err != nil {
			return fmt.Errorf("unable to create chart repo: %w", err)
		}
		chrepo.CachePath = h.tmpdir
		_, err = chrepo.DownloadIndexFile()
		if err != nil {
			return fmt.Errorf("unable to download index file: %w", err)
		}
	}
	h.repocfg = repocfg
	return nil
}

func (h *Helm) Close() error {
	return os.RemoveAll(h.tmpdir)
}

func (h *Helm) Latest(reponame, chart string) (string, error) {
	logrus.Infof("finding latest chart version of %s/%s", reponame, chart)
	for _, repository := range repositories.Repositories {
		if repository.Name != reponame {
			continue
		}
		chrepo, err := repo.NewChartRepository(repository, getters)
		if err != nil {
			return "", fmt.Errorf("unable to create chart repo: %w", err)
		}
		chrepo.CachePath = h.tmpdir
		idx, err := chrepo.DownloadIndexFile()
		if err != nil {
			return "", fmt.Errorf("unable to download index file: %w", err)
		}

		repoidx, err := repo.LoadIndexFile(idx)
		if err != nil {
			return "", fmt.Errorf("unable to load index file: %w", err)
		}

		versions, ok := repoidx.Entries[chart]
		if !ok {
			return "", fmt.Errorf("chart %s not found", chart)
		} else if len(versions) == 0 {
			return "", fmt.Errorf("chart %s has no versions", chart)
		}

		if len(versions) == 0 {
			return "", fmt.Errorf("chart %s has no versions", chart)
		}
		return versions[0].Version, nil
	}
	return "", fmt.Errorf("repository %s not found", reponame)
}

func (h *Helm) Pull(repo, chart, version string) (string, error) {
	if err := h.prepare(); err != nil {
		return "", err
	}

	dl := downloader.ChartDownloader{
		Out:              io.Discard,
		Options:          []getter.Option{},
		RepositoryConfig: h.repocfg,
		RepositoryCache:  h.tmpdir,
		Getters:          getters,
	}
	ref := fmt.Sprintf("%s/%s", repo, chart)
	dst, _, err := dl.DownloadTo(ref, version, os.TempDir())
	if err != nil {
		return "", err
	}
	return dst, nil
}

func (h *Helm) RegistryAuth(server, user, pass string) error {
	return h.regcli.Login(server, registry.LoginOptBasicAuth(user, pass))
}

func (h *Helm) Push(path, dst string) error {
	up := uploader.ChartUploader{
		Out:     os.Stdout,
		Pushers: pushers,
		Options: []pusher.Option{pusher.WithRegistryClient(h.regcli)},
	}
	return up.UploadTo(path, dst)
}

func (h *Helm) Render(chartName string, chartPath string, vals map[string]interface{}, namespace string) ([]string, error) {
	cfg := &action.Configuration{}

	client := action.NewInstall(cfg)
	client.DryRun = true
	client.ReleaseName = chartName
	client.Replace = true
	client.ClientOnly = true
	client.IncludeCRDs = true
	client.Namespace = namespace

	rawver, err := GetMakefileVariable("K0S_VERSION")
	if err != nil {
		return nil, fmt.Errorf("unable to get k0s version: %w", err)
	}
	kversion := semver.MustParse(rawver)

	// since ClientOnly is true we need to initialize KubeVersion otherwise resorts defaults
	client.KubeVersion = &chartutil.KubeVersion{
		Version: fmt.Sprintf("v%d.%d.0", kversion.Major(), kversion.Minor()),
		Major:   fmt.Sprintf("%d", kversion.Major()),
		Minor:   fmt.Sprintf("%d", kversion.Minor()),
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart: %w", err)
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			return nil, fmt.Errorf("failed dependency check: %w", err)
		}
	}

	rel, err := client.Run(chartRequested, vals)
	if err != nil {
		return nil, fmt.Errorf("failed to render chart: %w", err)
	}

	var manifests bytes.Buffer
	fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))
	for _, m := range rel.Hooks {
		fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)
	}

	resources := []string{}
	splitManifests := releaseutil.SplitManifests(manifests.String())
	for _, manifest := range splitManifests {
		manifest = strings.TrimSpace(manifest)
		resources = append(resources, manifest)
	}

	return resources, nil
}
