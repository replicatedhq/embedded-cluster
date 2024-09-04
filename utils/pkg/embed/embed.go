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
	dashes                = "-----" // this is broken up within the binary to prevent false positives
	beginReleaseDelimiter = "BEGIN APP RELEASE"
	endReleaseDelimiter   = "END APP RELEASE"
)

// EmbedReleaseDataInBinary embeds the release data in the binary at the end of the file and
// writes the new binary to the output path.
func EmbedReleaseDataInBinary(binPath string, releasePath string, outputPath string) error {
	binContent, err := os.ReadFile(binPath)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	// in arm64 binaries, the delimiters will already be part of the binary content in plain text,
	// so we need to check if the binary content _ends_ with the end delimiter in order to
	// determine if a release data is already embedded in the binary.
	if bytes.HasSuffix(binContent, delimiterBytes(endReleaseDelimiter)) {
		start := lastIndexOfDelimiter(binContent, beginReleaseDelimiter)
		end := lastIndexOfDelimiter(binContent, endReleaseDelimiter)
		binContent = append(binContent[:start], binContent[end+lengthOfDelimiter(endReleaseDelimiter):]...)
	}

	binReader := bytes.NewReader(binContent)
	binSize := int64(len(binContent))

	releaseData, err := os.ReadFile(releasePath)
	if err != nil {
		return fmt.Errorf("failed to read release data: %w", err)
	}

	newBinReader, totalLen := EmbedReleaseDataInBinaryReader(binReader, binSize, releaseData)
	newBinContent, err := io.ReadAll(newBinReader)
	if err != nil {
		return fmt.Errorf("failed to read new binary: %w", err)
	}
	if totalLen != int64(len(newBinContent)) {
		return fmt.Errorf("failed to read new binary: expected %d bytes, got %d", totalLen, len(newBinContent))
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
	newBinSize += int64(lengthOfDelimiter(beginReleaseDelimiter))
	newBinSize += int64(len(encodedRelease))
	newBinSize += int64(lengthOfDelimiter(endReleaseDelimiter))

	newBinReader := io.MultiReader(
		binReader,
		bytes.NewReader(delimiterBytes(beginReleaseDelimiter)),
		strings.NewReader(encodedRelease),
		bytes.NewReader(delimiterBytes(endReleaseDelimiter)),
	)

	return newBinReader, newBinSize
}

// ExtractReleaseDataFromBinary extracts the release data from the binary.
func ExtractReleaseDataFromBinary(exe string) ([]byte, error) {
	binContent, err := os.ReadFile(exe)
	if err != nil {
		return nil, fmt.Errorf("failed to read executable: %w", err)
	}

	start := lastIndexOfDelimiter(binContent, beginReleaseDelimiter)
	if start == -1 {
		return nil, nil
	}

	end := lastIndexOfDelimiter(binContent, endReleaseDelimiter)
	if end == -1 {
		return nil, fmt.Errorf("failed to find end delimiter in executable")
	}

	if start+lengthOfDelimiter(beginReleaseDelimiter) > len(binContent) {
		return nil, fmt.Errorf("invalid start delimiter")
	} else if start+lengthOfDelimiter(beginReleaseDelimiter) > end {
		return nil, fmt.Errorf("start delimter after end delimter")
	} else if end > len(binContent) {
		return nil, fmt.Errorf("invalid end delimiter")
	}

	encoded := binContent[start+lengthOfDelimiter(beginReleaseDelimiter) : end]

	decoded, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return nil, fmt.Errorf("failed to decode release data: %w", err)
	}

	return decoded, nil
}

// the go compiler will optimize concatenation of bytes as a constant string which will cause
// ExtractReleaseDataFromBinary to fail because it will find the delimiter in the wrong place.
// This function is used to create a byte slice that will be used as a delimiter while working
// around this issue.
func delimiterBytes(delim string) []byte {
	return []byte(dashes + delim + dashes)
}

func lastIndexOfDelimiter(s []byte, delim string) int {
	return bytes.LastIndex(s, delimiterBytes(delim))
}

func lengthOfDelimiter(delim string) int {
	return len(delimiterBytes(delim))
}
