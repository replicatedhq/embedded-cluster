// Package plain implements prompts using the standard library.
package plain

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// Confirm asks for user for a "Yes" or "No" response. The default value
// is used if the user presses enter without typing neither Y nor N.
func Confirm(msg string, defvalue bool) bool {
	options := " [y/N]"
	if defvalue {
		options = " [Y/n]"
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s %s: ", msg, options)
		input, err := reader.ReadString('\n')
		if err != nil {
			logrus.Fatalf("unable to read input: %v", err)
		}
		input = strings.ToLower(strings.TrimSpace(input))
		switch input {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		case "":
			return defvalue
		default:
			logrus.Errorf("Invalid input: %s", input)
		}
	}
}

// PressEnter asks the user to press enter to continue.
func PressEnter(msg string) {
	fmt.Printf("%s ", msg)
	reader := bufio.NewReader(os.Stdin)
	if _, err := reader.ReadString('\n'); err != nil {
		logrus.Fatalf("unable to read input: %v", err)
	}
}

// Password asks the user for a password. We just forward the call to Input
// with required set to true.
func Password(msg string) string {
	return Input(msg, "", true)
}

// Select asks the user to select one of the provided options.
func Select(msg string, options []string, _ string) string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println(msg)
		for _, option := range options {
			fmt.Printf(" - %s\n", option)
		}
		fmt.Printf("Type one of the options above: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			logrus.Fatalf("unable to read input: %v", err)
		}
		input = strings.TrimSuffix(input, "\n")
		for _, option := range options {
			if input != option {
				continue
			}
			return input
		}
		logrus.Errorf("Invalid option %q", input)
	}
}

// Input asks the user for a string. If required is true then
// the string cannot be empty.
func Input(msg string, _ string, required bool) string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s ", msg)
		if input, err := reader.ReadString('\n'); err != nil {
			logrus.Fatalf("unable to read input: %v", err)
		} else if !required || input != "" {
			return strings.TrimSuffix(input, "\n")
		}
		logrus.Error("Input cannot be empty")
	}
}
