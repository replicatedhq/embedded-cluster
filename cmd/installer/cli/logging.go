package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// MaxLogFiles is the maximum number of log files we keep.
const MaxLogFiles = 100

// StdoutLogger is a Logrus hook for routing Info, Warn and Error logs to stdout and Fatal logs to
// stderr.
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
	if entry.Level == logrus.FatalLevel {
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
	writer.Fprint(output, message)
	return nil
}

// needsFileLogging filters out, based on command line argument, if we need to log to a file.
// we only log to a file when running as root as the log location is in a directory a regular
// user may not be able to write to.
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
	return os.Getuid() == 0
}

// trimLogDir removes the oldest log files if we have more than MaxLogFiles.
func trimLogDir() {
	dir := runtimeconfig.EmbeddedClusterLogsSubDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) <= MaxLogFiles {
		return
	}
	oldest := time.Now()
	var fname string
	for _, file := range entries {
		info, err := file.Info()
		if err != nil {
			return
		}
		if info.ModTime().After(oldest) {
			continue
		}
		oldest = info.ModTime()
		fname = file.Name()
	}
	os.Remove(runtimeconfig.PathToLog(fname))
}

// SetupLogging sets up the logging for the application. If the debug flag is set we print
// all to the screen otherwise we print to a log file.
func SetupLogging() {
	if !needsFileLogging() {
		logrus.SetOutput(io.Discard)
		logrus.AddHook(&StdoutLogger{})
		return
	}
	logrus.SetLevel(logrus.DebugLevel)
	fname := fmt.Sprintf("%s-%s.log", runtimeconfig.AppSlug(), time.Now().Format("20060102150405.000"))
	logpath := runtimeconfig.PathToLog(fname)
	logfile, err := os.OpenFile(logpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0400)
	if err != nil {
		logrus.Warnf("Unable to setup logging: %v", err)
		return
	}
	logrus.SetOutput(logfile)
	logrus.AddHook(&StdoutLogger{})
	logrus.Debugf("command line: %v", os.Args)
	logrus.Debugf("Embedded Cluster: %s, k0s: %s", versions.Version, versions.K0sVersion)

	setupCtrlLogging(logfile)

	trimLogDir()
}

// setupCtrlLogging sets up the logging for the controller-runtime package to the writer specified.
func setupCtrlLogging(w io.Writer) {
	k8slogger := ctrlzap.New(func(o *zap.Options) {
		o.DestWriter = w
	})
	ctrllog.SetLogger(k8slogger)
}
