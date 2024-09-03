package embed

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmbedReleaseDataInBinary(t *testing.T) {
	// Create temporary files for testing
	binFile, err := os.CreateTemp("", "test-bin")
	assert.NoError(t, err)
	defer os.Remove(binFile.Name())

	releaseFile, err := os.CreateTemp("", "test-release")
	assert.NoError(t, err)
	defer os.Remove(releaseFile.Name())

	outputFile, err := os.CreateTemp("", "test-output")
	assert.NoError(t, err)
	defer os.Remove(outputFile.Name())

	// Write test data to the files
	binContent := []byte("test binary content")
	_, err = binFile.Write(binContent)
	assert.NoError(t, err)

	releaseData := []byte("test release data")
	_, err = releaseFile.Write(releaseData)
	assert.NoError(t, err)

	err = EmbedReleaseDataInBinary(binFile.Name(), releaseFile.Name(), outputFile.Name())
	assert.NoError(t, err)

	// Encode the release data for comparison
	encodedRelease := base64.StdEncoding.EncodeToString(releaseData)

	// Verify the new binary content
	gotBinContent, err := os.ReadFile(outputFile.Name())
	assert.NoError(t, err)

	wantBinContent := append(binContent, beginReleaseDelimiterBytes()...)
	wantBinContent = append(wantBinContent, []byte(encodedRelease)...)
	wantBinContent = append(wantBinContent, endReleaseDelimiterBytes()...)

	assert.Equal(t, string(wantBinContent), string(gotBinContent))

	// Verify the new binary size
	gotBinSize := int64(len(gotBinContent))
	wantBinSize := int64(len(binContent)) + int64(len(beginReleaseDelimiterBytes())) + int64(len(encodedRelease)) + int64(len(endReleaseDelimiterBytes()))
	assert.Equal(t, wantBinSize, gotBinSize)

	// Extract and verify the embedded release data
	embeddedData, err := ExtractReleaseDataFromBinary(outputFile.Name())
	assert.NoError(t, err)

	assert.Equal(t, string(releaseData), string(embeddedData))
}

func TestNoReleaseData(t *testing.T) {
	// Create temporary files for testing
	binFile, err := os.CreateTemp("", "test-bin")
	assert.NoError(t, err)
	defer os.Remove(binFile.Name())

	// Verify that no error is returned when the binary does not contain release data
	_, err = ExtractReleaseDataFromBinary(binFile.Name())
	assert.NoError(t, err)
}

func Test_beginReleaseDelimiterBytes(t *testing.T) {
	assert.Equalf(t, []byte("-----BEGIN APP RELEASE-----"), beginReleaseDelimiterBytes(), "beginReleaseDelimiterBytes()")
}

func Test_endReleaseDelimiterBytes(t *testing.T) {
	assert.Equalf(t, []byte("-----END APP RELEASE-----"), endReleaseDelimiterBytes(), "beginReleaseDelimiterBytes()")
}
