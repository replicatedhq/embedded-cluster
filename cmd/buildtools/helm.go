package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/pusher"
	"helm.sh/helm/v3/pkg/registry"
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
	regcli, err := registry.NewClient()
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
	logrus.Infof("finding latest version of %s/%s", reponame, chart)
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

		var outerr error
		sort.Slice(versions, func(i, j int) bool {
			v1, err1 := normalizeVersion(versions[i].Version)
			v2, err2 := normalizeVersion(versions[j].Version)
			if err1 != nil || err2 != nil {
				if outerr == nil {
					outerr = fmt.Errorf("%s, %s", err1, err2)
				}
				return false
			}
			return v1.LessThan(*v2)
		})

		if outerr != nil {
			return "", fmt.Errorf("unable to sort versions: %w", outerr)
		}
		return versions[len(versions)-1].Version, nil
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
		Out:            os.Stdout,
		Pushers:        pushers,
		RegistryClient: h.regcli,
	}
	return up.UploadTo(path, dst)
}

// normalizeVersion is used to convert a semver from string into a semver.Version.
// If the version does not include a patch version this functions appends a .0 at
// the end of it before parsing. This is intended for sorting version strings.
func normalizeVersion(version string) (*semver.Version, error) {
	parts := strings.Split(version, ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	paddedVersion := strings.Join(parts, ".")
	return semver.NewVersion(paddedVersion)
}
