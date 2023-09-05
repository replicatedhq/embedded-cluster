package logging

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type LogrusFileHook struct {
	file      *os.File
	flag      int
	chmod     os.FileMode
	formatter *logrus.TextFormatter
}

var Debug bool

func NewLogrusFileHook(file string, flag int, chmod os.FileMode) (*LogrusFileHook, error) {
	plainFormatter := &logrus.TextFormatter{DisableColors: true}
	logFile, err := os.OpenFile(file, flag, chmod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook %v", err)
		return nil, err
	}

	return &LogrusFileHook{logFile, flag, chmod, plainFormatter}, err
}

func (hook *LogrusFileHook) Fire(entry *logrus.Entry) error {

	plainformat, err := hook.formatter.Format(entry)
	line := string(plainformat)
	_, err = hook.file.WriteString(line)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook(entry.String)%v", err)
		return err
	}

	return nil
}

func (hook *LogrusFileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

type StandardHook struct {
	Writer    io.Writer
	LogLevels []logrus.Level
	Formatter *logrus.TextFormatter
}

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

func (hook *StandardHook) Levels() []logrus.Level {
	return hook.LogLevels
}

func SetupLogging() {
	logrus.SetOutput(io.Discard)

	logrus.AddHook(&StandardHook{
		Writer:    os.Stderr,
		LogLevels: logrus.AllLevels,
		Formatter: &logrus.TextFormatter{ForceColors: true},
	})

	now := time.Now().Format("20060102150405")

	fileHook, err := NewLogrusFileHook("helmvm-"+now+".log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err == nil {
		logrus.AddHook(fileHook)
	}
}
