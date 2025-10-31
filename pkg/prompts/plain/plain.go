// Package plain implements prompts using the standard library.
package plain

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// Plain implements Prompt using the standard library.
type Plain struct {
	in  io.Reader
	out io.Writer
}

type Option func(p *Plain)

func New(opts ...Option) Plain {
	p := Plain{
		in:  os.Stdin,
		out: os.Stdout,
	}
	for _, opt := range opts {
		opt(&p)
	}
	return p
}

func WithIn(in io.Reader) Option {
	return func(p *Plain) {
		p.in = in
	}
}

func WithOut(out io.Writer) Option {
	return func(p *Plain) {
		p.out = out
	}
}

// Confirm asks for user for a "Yes" or "No" response. The default value
// is used if the user presses enter without typing neither Y nor N.
func (p Plain) Confirm(msg string, defvalue bool) (bool, error) {
	options := " [y/N]"
	if defvalue {
		options = " [Y/n]"
	}
	if p.in == nil {
		return defvalue, nil
	}
	reader := bufio.NewReader(p.in)
	for {
		fmt.Fprintf(p.out, "%s %s: ", msg, options)
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("read input: %w", err)
		}
		input = strings.ToLower(strings.TrimSpace(input))
		switch input {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		case "":
			return defvalue, nil
		default:
			logrus.Errorf("Invalid input: %s", input)
		}
	}
}

// Password asks the user for a password. We just forward the call to Input
// with required set to true.
func (p Plain) Password(msg string) (string, error) {
	return p.Input(msg, "", true)
}

// Input asks the user for a string. If required is true then
// the string cannot be empty.
func (p Plain) Input(msg string, defvalue string, required bool) (string, error) {
	if p.in == nil {
		return defvalue, nil
	}
	reader := bufio.NewReader(p.in)
	for {
		fmt.Fprintf(p.out, "%s ", msg)
		if input, err := reader.ReadString('\n'); err != nil {
			return "", fmt.Errorf("read input: %w", err)
		} else if !required || input != "" {
			return strings.TrimSuffix(input, "\n"), nil
		}
		logrus.Error("Input cannot be empty")
	}
}
