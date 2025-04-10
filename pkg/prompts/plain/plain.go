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
	p := Plain{}
	for _, opt := range opts {
		opt(&p)
	}
	if p.in == nil {
		p.in = os.Stdin
	}
	if p.out == nil {
		p.out = os.Stdout
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

// PressEnter asks the user to press enter to continue.
func (p Plain) PressEnter(msg string) error {
	fmt.Fprintf(p.out, "%s ", msg)
	reader := bufio.NewReader(p.in)
	if _, err := reader.ReadString('\n'); err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	return nil
}

// Password asks the user for a password. We just forward the call to Input
// with required set to true.
func (p Plain) Password(msg string) (string, error) {
	return p.Input(msg, "", true)
}

// Select asks the user to select one of the provided options.
func (p Plain) Select(msg string, options []string, _ string) (string, error) {
	reader := bufio.NewReader(p.in)
	for {
		fmt.Println(msg)
		for _, option := range options {
			fmt.Fprintf(p.out, " - %s\n", option)
		}
		fmt.Fprintf(p.out, "Type one of the options above: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read input: %w", err)
		}
		input = strings.TrimSuffix(input, "\n")
		for _, option := range options {
			if input != option {
				continue
			}
			return input, nil
		}
		logrus.Errorf("Invalid option %q", input)
	}
}

// Input asks the user for a string. If required is true then
// the string cannot be empty.
func (p Plain) Input(msg string, defvalue string, required bool) (string, error) {
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
