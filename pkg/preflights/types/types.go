package types

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
)

// Output is the output of a troubleshoot preflight check as returned by
// `preflight --format=json`. It just wraps a list of results, aka records,
// classified by status.
type Output struct {
	Warn []Record `json:"warn"`
	Pass []Record `json:"pass"`
	Fail []Record `json:"fail"`
}

// HasFail returns true if any of the preflight checks failed.
func (o Output) HasFail() bool {
	return len(o.Fail) > 0
}

// HasWarn returns true if any of the preflight checks returned a warning.
func (o Output) HasWarn() bool {
	return len(o.Warn) > 0
}

// PrintTable prints the preflight output in a table format.
func (o Output) PrintTable() {
	o.printTable()
}

// PrintTableWithoutInfo prints the preflight output in a table format without info results.
func (o Output) PrintTableWithoutInfo() {
	withoutInfo := Output{
		Warn: o.Warn,
		Fail: o.Fail,
	}

	withoutInfo.printTable()
}

// wrapText wraps the text and adds a line break after width characters.
func (o Output) wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}

	var line string
	var wrappedText string
	for _, word := range strings.Fields(text) {
		if len(line)+len(word)+1 > width {
			wrappedText += fmt.Sprintf("%s\n", line)
			line = word
			continue
		}
		if line != "" {
			line += " "
		}
		line += word
	}

	wrappedText += line
	return wrappedText
}

// maxWidth determines the maximum width of the terminal, if larger than 150
// characters then it returns 150.
func (o Output) maxWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 150
	} else if width > 150 {
		return 150
	}
	return width
}

func (o Output) printTable() {
	tb := table.NewWriter()
	tb.SetStyle(
		table.Style{
			Box:     table.BoxStyle{PaddingLeft: " ", PaddingRight: " "},
			Options: table.Options{DrawBorder: false, SeparateRows: false, SeparateColumns: false},
		},
	)

	maxwidth := o.maxWidth()
	tb.SetAllowedRowLength(maxwidth)
	for _, rec := range append(o.Fail, o.Warn...) {
		tb.AppendRow(table.Row{"•", o.wrapText(rec.Message, maxwidth-5)})
	}
	logrus.Infof("\n%s\n", tb.Render())
}

func (o Output) SaveToDisk(path string) error {
	// Store results on disk of the host that ran the preflights
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal preflight results: %w", err)
	}

	// If we ever want to store multiple preflight results
	// we can add a timestamp to the filename.
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("unable to write preflight results to %s: %w", path, err)
	}

	return nil
}

// OutputFromReader reads the provided reader and returns a Output
// object. Expects the reader to contain a valid JSON object.
func OutputFromReader(from io.Reader) (*Output, error) {
	result := &Output{}
	if err := json.NewDecoder(from).Decode(result); err != nil {
		return result, fmt.Errorf("unable to decode preflight output: %w", err)
	}
	return result, nil
}

// Record is a single record of a troubleshoot preflight check.
type Record struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}
