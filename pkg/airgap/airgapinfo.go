package airgap

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"sigs.k8s.io/yaml"
)

// AirgapInfoFromReader extracts the airgap metadata from the airgap file and returns it
func AirgapInfoFromReader(reader io.Reader) (metadata *kotsv1beta1.Airgap, err error) {
	// decompress tarball
	ungzip, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress airgap file: %w", err)
	}
	defer ungzip.Close()

	// iterate through tarball
	tarreader := tar.NewReader(ungzip)
	var nextFile *tar.Header
	for {
		nextFile, err = tarreader.Next()
		if err != nil {
			if err == io.EOF {
				return nil, errors.Wrapf(err, "airgap.yaml not found in airgap file")
			}
			return nil, errors.Wrapf(err, "failed to read airgap file")
		}

		if nextFile.Name == "airgap.yaml" {
			var contents []byte
			contents, err = io.ReadAll(tarreader)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to read airgap.yaml file within airgap file")
			}
			parsed := kotsv1beta1.Airgap{}

			err := yaml.Unmarshal(contents, &parsed)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to unmarshal airgap.yaml file within airgap file")
			}
			return &parsed, nil
		}
	}
}

func AirgapInfoFromPath(path string) (metadata *kotsv1beta1.Airgap, err error) {
	reader, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open airgap file: %w", err)
	}
	defer reader.Close()

	return AirgapInfoFromReader(reader)
}
