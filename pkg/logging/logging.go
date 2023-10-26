// Package logging manages setup of common logging interfaces and settings.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/sirupsen/logrus"
)

// LogrusFileHook is a hook to handle writing to a file.
type LogrusFileHook struct {
	file      *os.File
	flag      int
	chmod     os.FileMode
	formatter *logrus.TextFormatter
}

// Debug is a flag that indicates whether debug logging is enabled.
var Debug bool

// NewLogrusFileHook creates a new hook to write to a file.
func NewLogrusFileHook(file string, flag int, chmod os.FileMode) (*LogrusFileHook, error) {
	plainFormatter := &logrus.TextFormatter{DisableColors: true}
	logFile, err := os.OpenFile(file, flag, chmod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook %v", err)
		return nil, err
	}

	return &LogrusFileHook{logFile, flag, chmod, plainFormatter}, err
}

// Fire is called when a log event is fired.
func (hook *LogrusFileHook) Fire(entry *logrus.Entry) error {

	plainformat, err := hook.formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to parse line %v", err)
		return err
	}
	line := string(plainformat)
	_, err = hook.file.WriteString(line)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook(entry.String) %v", err)
		return err
	}

	return nil
}

// Levels returns the levels that this hook should be enabled for.
func (hook *LogrusFileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// StandardHook is a hook to handle writing to a file.
type StandardHook struct {
	Writer    io.Writer
	LogLevels []logrus.Level
	Formatter *logrus.TextFormatter
}

// Fire is called when a log event is fired.
func (hook *StandardHook) Fire(entry *logrus.Entry) error {
	text, err := hook.Formatter.Format(entry)
	if err != nil {
		return err
	}

	if !Debug && entry.Level == logrus.InfoLevel || entry.Level == logrus.DebugLevel {
		return nil
	}

	_, err = hook.Writer.Write(text)
	return err
}

// Levels returns the levels that this hook should be enabled for.
func (hook *StandardHook) Levels() []logrus.Level {
	return hook.LogLevels
}

// SetupLogging sets up the logging for the application.
func SetupLogging() {
	logrus.SetOutput(io.Discard)

	logrus.AddHook(&StandardHook{
		Writer:    os.Stderr,
		LogLevels: logrus.AllLevels,
		Formatter: &logrus.TextFormatter{ForceColors: true},
	})

	now := time.Now().Format("20060102150405")

	dir := defaults.EmbeddedClusterLogsSubDir()
	path := filepath.Join(dir, "embedded-cluster-"+now+".log")

	fileHook, err := NewLogrusFileHook(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err == nil {
		logrus.AddHook(fileHook)
	}
}
