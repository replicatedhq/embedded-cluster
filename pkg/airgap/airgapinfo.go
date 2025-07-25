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

type AirgapMetadata struct {
	AirgapInfo   *kotsv1beta1.Airgap
	K0sImageSize int64
}

// AirgapMetadataFromReader extracts the airgap metadata from the airgap file and returns it
func AirgapMetadataFromReader(reader io.Reader) (metadata *AirgapMetadata, err error) {
	metadata = &AirgapMetadata{}
	// decompress tarball
	ungzip, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("decompress airgap file: %w", err)
	}
	defer ungzip.Close()

	// iterate through tarball
	tarreader := tar.NewReader(ungzip)
	var nextFile *tar.Header
	for {
		nextFile, err = tarreader.Next()
		if err != nil {
			if err == io.EOF {
				break
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
			metadata.AirgapInfo = &parsed
		}

		if nextFile.Name == ECAiragapImagePath {
			metadata.K0sImageSize = nextFile.Size
		}
	}

	if metadata.K0sImageSize == 0 {
		return nil, errors.New(fmt.Sprintf("%s not found in airgap file", ECAiragapImagePath))
	}

	if metadata.AirgapInfo == nil {
		return nil, errors.New("airgap.yaml not found in airgap file")
	}

	return metadata, nil
}

func AirgapMetadataFromPath(path string) (metadata *AirgapMetadata, err error) {
	reader, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open airgap file: %w", err)
	}
	defer reader.Close()

	return AirgapMetadataFromReader(reader)
}
