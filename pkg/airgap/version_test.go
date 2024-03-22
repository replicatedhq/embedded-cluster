package airgap

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAirgapBundleVersions(t *testing.T) {
	tests := []struct {
		name             string
		airgapDir        string
		wantAppslug      string
		wantChannelid    string
		wantVersionlabel string
	}{
		{
			name:             "tiny-airgap-noimages",
			airgapDir:        "tiny-airgap-noimages",
			wantAppslug:      "laverya-tiny-airgap",
			wantChannelid:    "2dMrAqJjrPzfeNHv9bc0gCHh25N",
			wantVersionlabel: "0.1.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			dir, err := os.Getwd()
			req.NoError(err)
			t.Logf("Current working directory: %s", dir)
			airgapReader := createTarballFromDir(filepath.Join(dir, "testfiles", tt.airgapDir), nil)

			appSlug, channelID, versionLabel, err := ChannelReleaseMetadata(airgapReader)
			req.NoError(err)
			req.Equal(tt.wantAppslug, appSlug)
			req.Equal(tt.wantChannelid, channelID)
			req.Equal(tt.wantVersionlabel, versionLabel)
		})
	}
}

func createTarballFromDir(rootPath string, additionalFiles map[string][]byte) io.Reader {
	appTarReader, appWriter := io.Pipe()
	gWriter := gzip.NewWriter(appWriter)
	appTarWriter := tar.NewWriter(gWriter)
	go func() {
		err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if rootPath == path {
				return nil
			}
			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}
			header.Name = filepath.Base(path)
			err = appTarWriter.WriteHeader(header)
			if err != nil {
				return err
			}
			if !info.IsDir() {
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				_, err = io.Copy(appTarWriter, file)
				if err != nil {
					return err
				}
				err = file.Close()
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			appTarWriter.Close()
			appWriter.CloseWithError(err)
			return
		}
		for name, data := range additionalFiles {
			header := tar.Header{
				Name: name,
				Size: int64(len(data)),
			}
			err = appTarWriter.WriteHeader(&header)
			if err != nil {
				appTarWriter.Close()
				appWriter.CloseWithError(err)
				return
			}
			_, err = appTarWriter.Write(data)
			if err != nil {
				appTarWriter.Close()
				appWriter.CloseWithError(err)
				return
			}
		}
		err = appTarWriter.Close()
		if err != nil {
			appWriter.CloseWithError(err)
			return
		}
		err = gWriter.Close()
		if err != nil {
			appWriter.CloseWithError(err)
			return
		}
		err = appWriter.Close()
		if err != nil {
			return
		}
	}()

	return appTarReader
}
