// Package logging manages setup of common logging interfaces and settings. We set the log
// level to all levels but we only show on stdout the info, error, and fatal levels. All
// other error levels are written only to a log file.
package logging

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

// StdoutLogger is a Logrus hook for routing Info, Error, and Fatal logs to the stdout.
type StdoutLogger struct{}

// Levels defines on which log levels this hook would trigger.
func (hook *StdoutLogger) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.InfoLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
	}
}

// Fire executes the hook for the given entry
func (hook *StdoutLogger) Fire(entry *logrus.Entry) error {
	message := fmt.Sprintf("%s\n", entry.Message)
	os.Stdout.Write([]byte(message))
	return nil
}

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
