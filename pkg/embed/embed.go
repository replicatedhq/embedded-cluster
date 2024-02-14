package embed

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	beginReleaseDelimiter = "-----BEGIN APP RELEASE-----"
	endReleaseDelimiter   = "-----END APP RELEASE-----"
)

// EmbedReleaseDataInBinary embeds the release data in the binary at the end of the file and
// writes the new binary to the output path.
func EmbedReleaseDataInBinary(binPath string, releasePath string, outputPath string) error {
	binContent, err := os.ReadFile(binPath)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	start := bytes.Index(binContent, []byte(beginReleaseDelimiter))
	end := bytes.Index(binContent, []byte(endReleaseDelimiter))

	if start != -1 && end != -1 {
		// some release data is already embedded in the binary, remove it
		binContent = append(binContent[:start], binContent[end+len(endReleaseDelimiter):]...)
	}

	binReader := bytes.NewReader(binContent)
	binSize := int64(len(binContent))

	releaseData, err := os.ReadFile(releasePath)
	if err != nil {
		return fmt.Errorf("failed to read release data: %w", err)
	}

	newBinReader, _ := EmbedReleaseDataInBinaryReader(binReader, binSize, releaseData)
	newBinContent, err := io.ReadAll(newBinReader)
	if err != nil {
		return fmt.Errorf("failed to read new binary: %w", err)
	}

	if err := os.WriteFile(outputPath, newBinContent, 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

// EmbedReleaseDataInBinaryReader embeds the release data in the binary at the end of the binary reader,
// and returns a new binary reader with the embedded release data and the new binary size.
func EmbedReleaseDataInBinaryReader(binReader io.Reader, binSize int64, releaseData []byte) (io.Reader, int64) {
	encodedRelease := base64.StdEncoding.EncodeToString(releaseData)

	newBinSize := binSize
	newBinSize += int64(len(beginReleaseDelimiter))
	newBinSize += int64(len(encodedRelease))
	newBinSize += int64(len(endReleaseDelimiter))

	newBinReader := io.MultiReader(
		binReader,
		strings.NewReader(beginReleaseDelimiter),
		strings.NewReader(encodedRelease),
		strings.NewReader(endReleaseDelimiter),
	)

	return newBinReader, newBinSize
}

// ExtractReleaseDataFromBinary extracts the release data from the binary.
func ExtractReleaseDataFromBinary(exe string) ([]byte, error) {
	binContent, err := os.ReadFile(exe)
	if err != nil {
		return nil, fmt.Errorf("failed to read executable: %w", err)
	}

	start := bytes.Index(binContent, []byte(beginReleaseDelimiter))
	if start == -1 {
		return nil, nil
	}

	end := bytes.Index(binContent, []byte(endReleaseDelimiter))
	if end == -1 {
		return nil, fmt.Errorf("failed to find end delimiter in executable")
	}

	encoded := binContent[start+len(beginReleaseDelimiter) : end]

	decoded, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return nil, fmt.Errorf("failed to decode release data: %w", err)
	}

	return decoded, nil
}
