package preflights

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/jedib0t/go-pretty/table"
	"github.com/replicatedhq/embedded-cluster/sdk/defaults"
	"github.com/sirupsen/logrus"
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

func (o Output) printTable() {
	tb := table.NewWriter()
	add := tb.AppendRow
	tb.AppendHeader(table.Row{"Status", "Title", "Message"})
	for _, rec := range o.Pass {
		add(table.Row{"PASS", rec.Title, rec.Message})
	}
	for _, rec := range o.Warn {
		add(table.Row{"WARN", rec.Title, rec.Message})
	}
	for _, rec := range o.Fail {
		add(table.Row{"FAIL", rec.Title, rec.Message})
	}
	tb.SortBy([]table.SortBy{{Name: "Status", Mode: table.Asc}})
	logrus.Infof("%s\n", tb.Render())
}

func (o Output) SaveToDisk() error {
	// Store results on disk of the host that ran the preflights
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal preflight results: %w", err)
	}

	// If we ever want to store multiple preflight results
	// we can add a timestamp to the filename.
	path := defaults.PathToEmbeddedClusterSupportFile("host-preflight-results.json")
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
