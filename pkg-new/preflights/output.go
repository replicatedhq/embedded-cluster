package preflights

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
)

// PrintTable prints the preflight output in a table format.
func (p *PreflightsRunner) PrintTable(o *apitypes.HostPreflightsOutput) {
	printTable(o)
}

// PrintTableWithoutInfo prints the preflight output in a table format without info results.
func (p *PreflightsRunner) PrintTableWithoutInfo(o *apitypes.HostPreflightsOutput) {
	withoutInfo := apitypes.HostPreflightsOutput{
		Warn: o.Warn,
		Fail: o.Fail,
	}

	printTable(&withoutInfo)
}

func (p *PreflightsRunner) SaveToDisk(o *apitypes.HostPreflightsOutput, path string) error {
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
func (p *PreflightsRunner) OutputFromReader(from io.Reader) (*apitypes.HostPreflightsOutput, error) {
	result := &apitypes.HostPreflightsOutput{}
	if err := json.NewDecoder(from).Decode(result); err != nil {
		return result, fmt.Errorf("unable to decode preflight output: %w", err)
	}
	return result, nil
}

// wrapText wraps the text and adds a line break after width characters.
func wrapText(text string, width int) string {
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
func maxWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 150
	} else if width > 150 {
		return 150
	}
	return width
}

func printTable(o *apitypes.HostPreflightsOutput) {
	tb := table.NewWriter()
	tb.SetStyle(
		table.Style{
			Box:     table.BoxStyle{PaddingLeft: " ", PaddingRight: " "},
			Options: table.Options{DrawBorder: false, SeparateRows: false, SeparateColumns: false},
		},
	)

	maxwidth := maxWidth()
	tb.SetAllowedRowLength(maxwidth)
	for _, rec := range append(o.Fail, o.Warn...) {
		tb.AppendRow(table.Row{"•", wrapText(rec.Message, maxwidth-5)})
	}
	logrus.Infof("\n%s\n", tb.Render())
}
