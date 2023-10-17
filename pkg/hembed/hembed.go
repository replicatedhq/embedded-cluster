// Package hembed manages the helm chart embedding mechanism. It is used when the user
// wants to embed a custom helm chart into the helmvm binary. Writes the chart and adds
// a mark to the end of the file. The mark is on the format HELMVMCHARTS0000000000 where
// the number is the length of the embedded data.
package hembed

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	BaseDir = "/helmvm"
	EndMark = "HELMVMCHARTS"
)

// Binary is a helpes struct that holds a closer and a reader separately. Reads happen
// on the reader and Closes happen on the closer.
type Binary struct {
	closer io.Closer
	reader io.Reader
	size   int64
}

// Size returns the total size of the binary.
func (b *Binary) Size() int64 {
	return b.size
}

// Close closes the internal closer.
func (b *Binary) Close() error {
	return b.closer.Close()
}

// Read reads from the internal reader.
func (b *Binary) Read(p []byte) (n int, err error) {
	return b.reader.Read(p)
}

// HelmChart represents a helm chart. It has a base64 encoded content. Images that are used
// by the chart can be passed in the Images field if they need to be pulled for disconnected
// installs.
type HelmChart struct {
	Content string `yaml:"content"`
	Values  string `yaml:"values"`
}

// ChartReader returns an io.Reader that can be used to read the contents of the helm chart.
// Content is decoded from base64.
func (c *HelmChart) ChartReader() io.Reader {
	return base64.NewDecoder(base64.StdEncoding, strings.NewReader(c.Content))
}

// ValidateTarget checks we have a binary for the target OS and Arch on the disk, under
// the BaseDir directory.
func ValidateTarget(opts EmbedOptions) error {
	fpath := PathToPrebuiltBinary(opts)
	if _, err := os.Stat(fpath); err == nil {
		return nil
	}
	return fmt.Errorf("target %s/%s is not supported", opts.OS, opts.Arch)
}

// EmbedOptions are the options for embedding helmcharts into helmvm.
type EmbedOptions struct {
	OS     string
	Arch   string
	Charts []HelmChart
}

// CalculateRewind calculates how far back we need to rewind inside the file to
// start reading the embedded data. Returns the number of bytes to rewind and the
// size of the embedded data to be read.
func CalculateRewind(fpath string) (int64, int64, error) {
	fp, err := os.Open(fpath)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to open file %s: %w", fpath, err)
	}
	defer fp.Close()
	placeHolder := []byte(fmt.Sprintf("%s%010d", EndMark, 0))
	ruler := bytes.NewBuffer(placeHolder)
	if _, err = fp.Seek(-int64(ruler.Len()), io.SeekEnd); err != nil {
		return 0, 0, fmt.Errorf("unable to seek to end of file: %w", err)
	}
	buf := make([]byte, ruler.Len())
	if _, err = fp.Read(buf); err != nil {
		return 0, 0, fmt.Errorf("unable to read end of file: %w", err)
	}
	if !strings.HasPrefix(string(buf), EndMark) {
		return 0, 0, nil
	}
	strbuf := strings.TrimPrefix(string(buf), EndMark)
	size, err := strconv.ParseInt(strbuf, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to parse end of file: %w", err)
	}
	rewind := size + int64(ruler.Len())
	return rewind, size, nil
}

// ReadEmbeddedData reads the embedded data from the binary. It reads the binary from
// the disk and looks for a mark at the end of the file. If the mark is found, it reads
// the data that is embedded in the binary and returns it. The mark is on the format
// HELMVMCHARTS0000000000 where the number is the length of the embedded data.
func ReadEmbeddedData(fpath string) ([]byte, error) {
	fp, err := os.Open(fpath)
	if err != nil {
		return nil, fmt.Errorf("unable to open executable: %w", err)
	}
	defer fp.Close()
	rewind, size, err := CalculateRewind(fpath)
	if err != nil {
		return nil, fmt.Errorf("unable to calculate rewind: %w", err)
	}
	if rewind == 0 {
		return nil, nil
	}
	if _, err = fp.Seek(-int64(rewind), io.SeekEnd); err != nil {
		return nil, fmt.Errorf("unable to seek to end of file: %w", err)
	}
	data := make([]byte, size)
	if _, err = fp.Read(data); err != nil {
		return nil, fmt.Errorf("unable to read end of file: %w", err)
	}
	gzreader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("unable to create gzip reader: %w", err)
	}
	out := bytes.NewBuffer(nil)
	if _, err = io.Copy(out, gzreader); err != nil {
		return nil, fmt.Errorf("unable to read embedded data: %w", err)
	}
	return out.Bytes(), nil
}

// PathToPrebuiltBinary return the path fo the helmvm binary for the given OS and Arch.
// This function is meant to be used inside the "builder" container.
func PathToPrebuiltBinary(opts EmbedOptions) string {
	fname := fmt.Sprintf("helmvm-%s-%s", opts.OS, opts.Arch)
	return path.Join(BaseDir, fname)
}

// ReadEmbedOptionsFromBinary reads the embedded charts from the binary. It reads the
// binary from the disk and looks for a mark at the end of the file.
func ReadEmbedOptionsFromBinary(fpath string) (*EmbedOptions, error) {
	data, err := ReadEmbeddedData(fpath)
	if err != nil {
		return nil, fmt.Errorf("unable to read embedded data: %w", err)
	} else if data == nil {
		return nil, nil
	}
	opts := &EmbedOptions{}
	if err := yaml.Unmarshal(data, opts); err != nil {
		return nil, fmt.Errorf("unable to unmarshal embedded data: %w", err)
	}
	return opts, nil
}

// Embed embeds helmcharts into the helmvm binary.
func Embed(ctx context.Context, fpath string, opts EmbedOptions) (*Binary, error) {
	request := bytes.NewBuffer(nil)
	gwriter := gzip.NewWriter(request)
	ymlenc := yaml.NewEncoder(gwriter)
	if err := ymlenc.Encode(opts); err != nil {
		gwriter.Close()
		return nil, fmt.Errorf("unable to encode charts: %w", err)
	}
	gwriter.Close()
	// we do not accept anything longer than 100MB.
	if request.Len() > 100*1024*1024 {
		gwriter.Close()
		return nil, fmt.Errorf("unable to encode: request too large")
	}
	binary, err := os.Open(fpath)
	if err != nil {
		return nil, fmt.Errorf("unable to open binary: %w", err)
	}
	stat, err := binary.Stat()
	if err != nil {
		return nil, fmt.Errorf("unable to stat binary: %w", err)
	}
	rewind, _, err := CalculateRewind(fpath)
	if err != nil {
		return nil, fmt.Errorf("unable to calculate rewind: %w", err)
	}
	binreader := io.LimitReader(binary, stat.Size()-rewind)
	end := fmt.Sprintf("%s%010d", EndMark, request.Len())
	endreader := strings.NewReader(end)
	return &Binary{
		closer: binary,
		reader: io.MultiReader(binreader, request, endreader),
		size:   stat.Size() + int64(request.Len()) + int64(endreader.Len()),
	}, nil
}
