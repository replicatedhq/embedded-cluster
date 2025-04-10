// Package decorative implement decorative prompts using the survey library.
package decorative

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
)

// Decorative is a decorative prompt.
type Decorative struct{}

func New() Decorative {
	return Decorative{}
}

// Confirm asks for user for a "Yes" or "No" response. The default value
// is used if the user presses enter without typing anything.
func (d Decorative) Confirm(msg string, defvalue bool) (bool, error) {
	var response bool
	var confirm = &survey.Confirm{Message: msg, Default: defvalue}
	if err := survey.AskOne(confirm, &response); err != nil {
		return false, fmt.Errorf("unable to confirm: %w", err)
	}
	return response, nil
}

// PressEnter asks the user to press enter to continue.
func (d Decorative) PressEnter(msg string) error {
	var i string
	in := &survey.Input{Message: msg}
	if err := survey.AskOne(in, &i); err != nil {
		return fmt.Errorf("unable to ask for input: %w", err)
	}
	return nil
}

// Password asks the user for a password. Password can't be empty.
func (d Decorative) Password(msg string) (string, error) {
	var pass string
	for pass == "" {
		question := &survey.Password{Message: msg}
		if err := survey.AskOne(question, &pass); err != nil {
			return "", fmt.Errorf("unable to ask for input: %w", err)
		} else if pass == "" {
			logrus.Error("Password cannot be empty")
		}
	}
	return pass, nil
}

// Select asks the user to select one of the provided options.
func (d Decorative) Select(msg string, options []string, defvalue string) (string, error) {
	question := &survey.Select{
		Message: msg,
		Options: options,
		Default: defvalue,
	}
	var response string
	if err := survey.AskOne(question, &response); err != nil {
		return "", fmt.Errorf("unable to ask for input: %w", err)
	}
	return response, nil
}

// Input asks the user for a string. If required is true then
// the string cannot be empty.
func (d Decorative) Input(msg string, defvalue string, required bool) (string, error) {
	var response string
	for response == "" {
		question := &survey.Input{Message: msg, Default: defvalue}
		if err := survey.AskOne(question, &response); err != nil {
			return "", fmt.Errorf("unable to ask for input: %w", err)
		} else if !required || response != "" {
			break
		}
		logrus.Error("Input cannot be empty")
	}
	return response, nil
}
