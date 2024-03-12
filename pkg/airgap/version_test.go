package airgap

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
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

			appSlug, channelID, versionLabel, err := AirgapBundleVersions(airgapReader)
			req.NoError(err)
			req.Equal(tt.wantAppslug, appSlug)
			req.Equal(tt.wantChannelid, channelID)
			req.Equal(tt.wantVersionlabel, versionLabel)
		})
	}
}
