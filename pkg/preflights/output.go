package preflights

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/jedib0t/go-pretty/table"
)

// Outputs holds a list of Output objects indexed by host address.
type Outputs map[string]*Output

// HaveFails returns true if any of the host preflight checks failed.
func (o Outputs) HaveFails() bool {
	for _, out := range o {
		if out.HasFail() {
			return true
		}
	}
	return false
}

// HaveWarns returns true if any of the host preflight checks returned
// a warning.
func (o Outputs) HaveWarns() bool {
	for _, out := range o {
		if out.HasWarn() {
			return true
		}
	}
	return false
}

// PrintTable prints the preflight output in a table format.
func (o Outputs) PrintTable() {
	tb := table.NewWriter()
	add := tb.AppendRow
	tb.AppendHeader(table.Row{"Address", "Status", "Title", "Message"})
	for addr, out := range o {
		for _, rec := range out.Pass {
			add(table.Row{addr, "PASS", rec.Title, rec.Message})
		}
		for _, rec := range out.Warn {
			add(table.Row{addr, "WARN", rec.Title, rec.Message})
		}
		for _, rec := range out.Fail {
			add(table.Row{addr, "FAIL", rec.Title, rec.Message})
		}
	}
	tb.SortBy([]table.SortBy{{Name: "Status", Mode: table.Asc}})
	fmt.Printf("%s\n", tb.Render())
}

// NewOutputs creates a new Outputs object.
func NewOutputs() Outputs {
	return make(map[string]*Output)
}

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
	fmt.Printf("%s\n", tb.Render())
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
