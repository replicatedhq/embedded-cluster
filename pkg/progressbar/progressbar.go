// Package progressbar provides a simple progress bar for the CLI. This
// should have been called "loadingbar" instead as there is no progress
// at all.
package progressbar

import (
	"fmt"
	"strings"
	"time"
)

var blocks = []string{"◐", "◓", "◑", "◒"}

// WriterFn is a function that writes a formatted string.
type WriteFn func(string, ...any) (int, error)

// MessageWriter implements io.Writer on top of a channel of strings.
type MessageWriter struct {
	ch     chan string
	end    chan struct{}
	printf WriteFn
}

// Write implements io.Writer for the MessageWriter.
func (m MessageWriter) Write(p []byte) (n int, err error) {
	str := string(p)
	lines := strings.Split(str, "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		m.ch <- line
	}
	return len(p), nil
}

// Closef closes the MessageWriter after writing a message.
func (m MessageWriter) Closef(format string, args ...interface{}) {
	m.ch <- fmt.Sprintf(format, args...)
	m.Close()
}

// Close closes the MessageWriter inner channel.
func (m MessageWriter) Close() {
	close(m.ch)
	<-m.end
}

// Tracef is implemeted to comply with rig log.Logger interface.
func (m MessageWriter) Tracef(format string, args ...interface{}) {
	m.ch <- fmt.Sprintf(format, args...)
}

// Debugf is implemeted to comply with rig log.Logger interface.
func (m MessageWriter) Debugf(format string, args ...interface{}) {
	m.ch <- fmt.Sprintf(format, args...)
}

// Infof is implemeted to comply with rig log.Logger interface.
func (m MessageWriter) Infof(format string, args ...interface{}) {
	m.ch <- fmt.Sprintf(format, args...)
}

// Warnf is implemeted to comply with rig log.Logger interface.
func (m MessageWriter) Warnf(format string, args ...interface{}) {
	m.ch <- fmt.Sprintf(format, args...)
}

// Errorf is implemeted to comply with rig log.Logger interface.
func (m MessageWriter) Errorf(format string, args ...interface{}) {
	m.ch <- fmt.Sprintf(format, args...)
}

// loop keeps reading messages from the channel and printint them
// using the provided WriteFn. Exits when the channel is closed.
func (m MessageWriter) loop() {
	var counter int
	var message string
	var ticker = time.NewTicker(50 * time.Millisecond)
	var end bool
	for {
		select {
		case msg, open := <-m.ch:
			if !open {
				end = true
			} else {
				message = msg
			}
		case <-ticker.C:
			counter++
		}

		pos := counter % len(blocks)
		if !end {
			_, _ = m.printf("\033[K\r%s  %s ", blocks[pos], message)
			continue
		}
		_, _ = m.printf("\033[K\r✓  %s\n", message)
		close(m.end)
		return
	}
}

// Start starts the progress bar. Returns a writer the caller can use to
// send us messages to be printed using the provided WriteFn (if nil then
// we print to stdout). MessageWriter should be closed when done.
// MessageWriter is closed.
func Start(fn WriteFn) MessageWriter {
	if fn == nil {
		fn = fmt.Printf
	}
	mw := MessageWriter{
		ch:     make(chan string, 1024),
		end:    make(chan struct{}),
		printf: fn,
	}
	go mw.loop()
	return mw
}
