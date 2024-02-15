// Package logging manages setup of common logging interfaces and settings. We set the log
// level to all levels but we only show on stdout the info, error, and fatal levels. All
// other error levels are written only to a log file.
package logging

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

// StdoutLogger is a Logrus hook for routing Info, Error, Warn, and Fatal logs to the screen.
type StdoutLogger struct{}

// Levels defines on which log levels this hook would trigger.
func (hook *StdoutLogger) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
	}
}

// Fire executes the hook for the given entry.
func (hook *StdoutLogger) Fire(entry *logrus.Entry) error {
	message := fmt.Sprintf("%s\n", entry.Message)
	output := os.Stdout
	if entry.Level != logrus.InfoLevel {
		output = os.Stderr
	}
	var writer *color.Color
	switch entry.Level {
	case logrus.WarnLevel:
		writer = color.New(color.FgYellow)
	case logrus.ErrorLevel, logrus.FatalLevel:
		writer = color.New(color.FgRed)
	default:
		writer = color.New(color.FgWhite)
	}
	writer.Fprintf(output, message)
	return nil
}

// needsFileLogging filters out, based on command line argument, if we need to log to a file.
func needsFileLogging() bool {
	if len(os.Args) == 1 {
		return false
	}
	cmdline := strings.Join(os.Args, " ")
	if strings.Contains(cmdline, "version") {
		return false
	}
	if strings.Contains(cmdline, "help") {
		return false
	}
	if strings.Contains(cmdline, "shell") {
		return false
	}
	return true
}

// SetupLogging sets up the logging for the application. If the debug flag is set we print
// all to the screen otherwise we print to a log file.
func SetupLogging() {
	if !needsFileLogging() {
		return
	}
	logrus.SetLevel(logrus.DebugLevel)
	fname := fmt.Sprintf("%s-%s.log", defaults.BinaryName(), time.Now().Format("2006-01-02-15:04:05.000"))
	logpath := defaults.PathToLog(fname)
	logfile, err := os.OpenFile(logpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0400)
	if err != nil {
		logrus.Warnf("unable to setup logging: %v", err)
		return
	}
	logrus.SetOutput(logfile)
	logrus.AddHook(&StdoutLogger{})
	logrus.Debugf("command line: %v", os.Args)
}
