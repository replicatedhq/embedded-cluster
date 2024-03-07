package airgap

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/gosimple/slug"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateAppConfigMaps(cli client.Client, airgapFile string) error {
	// read file from airgapFile
	rawfile, err := os.Open(airgapFile)
	if err != nil {
		return fmt.Errorf("failed to open airgap file: %w", err)
	}
	defer rawfile.Close()

	// decompress tarball
	ungzip, err := gzip.NewReader(rawfile)
	if err != nil {
		return fmt.Errorf("failed to decompress airgap file: %w", err)
	}

	// iterate through tarball
	tarreader := tar.NewReader(ungzip)
	var nextFile *tar.Header
	foundAppRelease, foundAirgapYaml := false, false
	for nextFile, err = tarreader.Next(); err != nil; {
		if nextFile.Name == "airgap.yaml" {
			foundAirgapYaml = true
			var contents []byte
			contents, err = io.ReadAll(tarreader)
			if err != nil {
				return fmt.Errorf("failed to read airgap.yaml file within %s: %w", airgapFile, err)
			}
			err = createAppConfigMap(context.Background(), cli, "airgap-meta", contents)
			if err != nil {
				return fmt.Errorf("failed to create app configmap: %w", err)
			}
		}
		if nextFile.Name == "app.tar.gz" {
			foundAppRelease = true
			err = createAppYamlConfigMaps(context.Background(), cli, tarreader)
			if err != nil {
				return fmt.Errorf("failed to read app release file within %s: %w", airgapFile, err)
			}
		}
		if foundAppRelease && foundAirgapYaml {
			break
		}
	}
	if !foundAppRelease {
		return fmt.Errorf("app release not found in %s", airgapFile)
	}
	if !foundAirgapYaml {
		return fmt.Errorf("airgap.yaml not found in %s", airgapFile)
	}

	return nil
}

func createAppYamlConfigMaps(ctx context.Context, cli client.Client, apptarball io.Reader) error {
	// read file from airgapFile
	// decompress tarball
	// iterate through tarball
	// create configmap each file in tarball

	ungzip, err := gzip.NewReader(apptarball)
	if err != nil {
		return fmt.Errorf("failed to decompress app release file: %w", err)
	}

	tarreader := tar.NewReader(ungzip)
	var nextFile *tar.Header
	for nextFile, err = tarreader.Next(); err != nil; {
		var contents []byte
		contents, err = io.ReadAll(tarreader)

		if err != nil {
			return fmt.Errorf("failed to read app release file %s: %w", nextFile.Name, err)
		}

		err = createAppConfigMap(ctx, cli, nextFile.Name, contents)
		if err != nil {
			return fmt.Errorf("failed to create app configmap: %w", err)
		}
	}

	return nil
}

func createAppConfigMap(ctx context.Context, cli client.Client, key string, contents []byte) error {
	rel, err := release.GetChannelRelease()
	if err != nil {
		return fmt.Errorf("failed to get channel release: %w", err)
	}

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("kotsadm-airgap-%s", slug.Make(key)),
			Namespace: defaults.KOTSADM_NAMESPACE,
			Labels: map[string]string{
				"kots.io/automation": "airgap",
				"kots.io/app":        rel.AppSlug,
				"kots.io/kotsadm":    "true",
			},
		},
		Data: map[string]string{
			key: base64.StdEncoding.EncodeToString(contents),
		},
	}

	err = cli.Create(ctx, configMap)
	if err != nil {
		return fmt.Errorf("failed to create configmap %s: %w", configMap.Name, err)
	}

	return nil
}
