// Package decorative implement decorative prompts using the survey library.
package decorative

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
)

// Confirm asks for user for a "Yes" or "No" response. The default value
// is used if the user presses enter without typing anything.
func Confirm(msg string, defvalue bool) bool {
	var response bool
	var confirm = &survey.Confirm{Message: msg, Default: defvalue}
	if err := survey.AskOne(confirm, &response); err != nil {
		logrus.Fatalf("unable to confirm: %v", err)
	}
	return response
}

// PressEnter asks the user to press enter to continue.
func PressEnter(msg string) {
	var i string
	if err := survey.AskOne(&survey.Input{Message: msg}, &i); err != nil {
		logrus.Fatalf("unable to ask for input: %v", err)
	}
}

// Password asks the user for a password. Password can't be empty.
func Password(msg string) string {
	var pass string
	for pass == "" {
		question := &survey.Password{Message: msg}
		if err := survey.AskOne(question, &pass); err != nil {
			logrus.Fatalf("unable to ask for input: %v", err)
		} else if pass == "" {
			logrus.Error("Password cannot be empty")
		}
	}
	return pass
}

// Select asks the user to select one of the provided options.
func Select(msg string, options []string, defvalue string) string {
	question := &survey.Select{
		Message: msg,
		Options: options,
		Default: defvalue,
	}
	var response string
	if err := survey.AskOne(question, &response); err != nil {
		logrus.Fatalf("unable to ask for input: %v", err)
	}
	return response
}

// Input asks the user for a string. If required is true then
// the string cannot be empty.
func Input(msg string, defvalue string, required bool) string {
	var response string
	for response == "" {
		question := &survey.Input{Message: msg, Default: defvalue}
		if err := survey.AskOne(question, &response); err != nil {
			logrus.Fatalf("unable to ask for input: %v", err)
		} else if !required || response != "" {
			break
		}
		logrus.Error("Input cannot be empty")
	}
	return response
}
