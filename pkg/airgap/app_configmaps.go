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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateAppConfigMaps(ctx context.Context, cli client.Client, airgapReader io.Reader) error {
	err := createNamespaceIfNotExist(ctx, cli, defaults.KOTSADM_NAMESPACE)
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	// decompress tarball
	ungzip, err := gzip.NewReader(airgapReader)
	if err != nil {
		return fmt.Errorf("failed to decompress airgap file: %w", err)
	}

	// iterate through tarball
	tarreader := tar.NewReader(ungzip)
	var nextFile *tar.Header
	foundAppRelease, foundAirgapYaml := false, false
	for {
		nextFile, err = tarreader.Next()
		if err != nil {
			if err == io.EOF {
				if !foundAppRelease {
					return fmt.Errorf("app release not found in airgap file")
				}
				if !foundAirgapYaml {
					return fmt.Errorf("airgap.yaml not found in airgap file")
				}
			}
			return fmt.Errorf("failed to read airgap file: %w", err)
		}

		if nextFile.Name == "airgap.yaml" {
			foundAirgapYaml = true
			var contents []byte
			contents, err = io.ReadAll(tarreader)
			if err != nil {
				return fmt.Errorf("failed to read airgap.yaml file within airgap file: %w", err)
			}
			err = createAppConfigMap(ctx, cli, "meta", "airgap.yaml", contents)
			if err != nil {
				return fmt.Errorf("failed to create app configmap: %w", err)
			}
		}
		if nextFile.Name == "app.tar.gz" {
			foundAppRelease = true
			err = createAppYamlConfigMaps(ctx, cli, tarreader)
			if err != nil {
				return fmt.Errorf("failed to read app release file within airgap file: %w", err)
			}
		}
		if foundAppRelease && foundAirgapYaml {
			break
		}
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
	for {
		nextFile, err = tarreader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read app release file: %w", err)
		}

		var contents []byte
		contents, err = io.ReadAll(tarreader)

		if err != nil {
			return fmt.Errorf("failed to read app release file %s: %w", nextFile.Name, err)
		}

		err = createAppConfigMap(ctx, cli, nextFile.Name, nextFile.Name, contents)
		if err != nil {
			return fmt.Errorf("failed to create app configmap: %w", err)
		}
	}

	return nil
}

func createNamespaceIfNotExist(ctx context.Context, cli client.Client, name string) error {
	existingNs := &corev1.Namespace{}

	err := cli.Get(ctx, client.ObjectKey{Name: name}, existingNs)
	if err == nil {
		return nil
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	err = cli.Create(ctx, ns)
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", ns.Name, err)
	}

	return nil
}

func createAppConfigMap(ctx context.Context, cli client.Client, name string, filename string, contents []byte) error {
	rel, err := release.GetChannelRelease()
	if err != nil {
		return fmt.Errorf("failed to get channel release: %w", err)
	}
	if rel == nil {
		rel = &release.ChannelRelease{
			AppSlug: "unknown",
		}
	}

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("kotsadm-airgap-%s", slug.Make(name)),
			Namespace: defaults.KOTSADM_NAMESPACE,
			Labels: map[string]string{
				"kots.io/automation": "airgap",
				"kots.io/app":        rel.AppSlug,
				"kots.io/kotsadm":    "true",
			},
		},
		Data: map[string]string{
			filename: base64.StdEncoding.EncodeToString(contents),
		},
	}

	err = cli.Create(ctx, configMap)
	if err != nil {
		return fmt.Errorf("failed to create configmap %s: %w", configMap.Name, err)
	}

	return nil
}
