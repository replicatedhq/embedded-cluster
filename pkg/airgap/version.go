package airgap

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"sigs.k8s.io/yaml"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// ChannelReleaseMetadata returns the appSlug, channelID, and versionLabel of the airgap bundle
func ChannelReleaseMetadata(reader io.Reader) (appSlug, channelID, versionLabel string, err error) {

	// decompress tarball
	ungzip, err := gzip.NewReader(reader)
	if err != nil {
		err = fmt.Errorf("failed to decompress airgap file: %w", err)
		return
	}

	// iterate through tarball
	tarreader := tar.NewReader(ungzip)
	var nextFile *tar.Header
	for {
		nextFile, err = tarreader.Next()
		if err != nil {
			if err == io.EOF {
				err = fmt.Errorf("app release not found in airgap file")
				return
			}
			err = fmt.Errorf("failed to read airgap file: %w", err)
			return
		}

		if nextFile.Name == "airgap.yaml" {
			var contents []byte
			contents, err = io.ReadAll(tarreader)
			if err != nil {
				err = fmt.Errorf("failed to read airgap.yaml file within airgap file: %w", err)
				return
			}
			var airgapInfo kotsv1beta1.Airgap
			airgapInfo, err = airgapYamlVersions(contents)
			if err != nil {
				err = fmt.Errorf("failed to parse airgap.yaml: %w", err)
				return
			}
			appSlug = airgapInfo.Spec.AppSlug
			channelID = airgapInfo.Spec.ChannelID
			versionLabel = airgapInfo.Spec.VersionLabel
			return
		}
	}
}

func airgapYamlVersions(contents []byte) (kotsv1beta1.Airgap, error) {
	parsed := kotsv1beta1.Airgap{}

	err := yaml.Unmarshal(contents, &parsed)
	if err != nil {
		return kotsv1beta1.Airgap{}, err
	}
	return parsed, nil
}
