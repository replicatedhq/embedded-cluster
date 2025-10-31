// Package decorative implement decorative prompts using the survey library.
package decorative

import (
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
		return false, err
	}
	return response, nil
}

// Password asks the user for a password. Password can't be empty.
func (d Decorative) Password(msg string) (string, error) {
	var pass string
	for pass == "" {
		question := &survey.Password{Message: msg}
		if err := survey.AskOne(question, &pass); err != nil {
			return "", err
		} else if pass == "" {
			logrus.Error("Password cannot be empty")
		}
	}
	return pass, nil
}

// Input asks the user for a string. If required is true then
// the string cannot be empty.
func (d Decorative) Input(msg string, defvalue string, required bool) (string, error) {
	var response string
	for response == "" {
		question := &survey.Input{Message: msg, Default: defvalue}
		if err := survey.AskOne(question, &response); err != nil {
			return "", err
		} else if !required || response != "" {
			break
		}
		logrus.Error("Input cannot be empty")
	}
	return response, nil
}
