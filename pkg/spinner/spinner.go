// Package spinner provides a simple spinner for the CLI.
package spinner

import (
	"fmt"
	"strings"
	"time"
)

var blocks = []string{"◐", "◓", "◑", "◒"}

// WriteFn is a function that writes a formatted string.
type WriteFn func(string, ...any) (int, error)

// LineBreakerFn is a function that determines if it is time to break the
// line thus creating a new spinner line. Returns a boolean and a string
// indicating what needs to be printed before the line break.
type LineBreakerFn func(string) (bool, string)

// MaskFn is a function that masks a message. Receives a string and
// returns a string, the returned string is printed to the terminal.
type MaskFn func(string) string

// MessageWriter implements io.Writer on top of a channel of strings.
type MessageWriter struct {
	ch     chan string
	end    chan struct{}
	err    bool
	printf WriteFn
	mask   MaskFn
	lbreak LineBreakerFn
}

// Write implements io.Writer for the MessageWriter.
func (m *MessageWriter) Write(p []byte) (n int, err error) {
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
func (m *MessageWriter) Closef(format string, args ...interface{}) {
	m.ch <- fmt.Sprintf(format, args...)
	m.Close()
}

// Close closes the MessageWriter inner channel.
func (m *MessageWriter) Close() {
	close(m.ch)
	<-m.end
}

// CloseWithError closes the MessageWriter with an error.
func (m *MessageWriter) CloseWithError() {
	m.err = true
	close(m.ch)
	<-m.end
}

// Tracef is implemeted to comply with rig log.Logger interface.
func (m *MessageWriter) Tracef(format string, args ...interface{}) {
	m.ch <- fmt.Sprintf(format, args...)
}

// Debugf is implemeted to comply with rig log.Logger interface.
func (m *MessageWriter) Debugf(format string, args ...interface{}) {
	m.ch <- fmt.Sprintf(format, args...)
}

// Infof is implemeted to comply with rig log.Logger interface.
func (m *MessageWriter) Infof(format string, args ...interface{}) {
	m.ch <- fmt.Sprintf(format, args...)
}

// Warnf is implemeted to comply with rig log.Logger interface.
func (m *MessageWriter) Warnf(format string, args ...interface{}) {
	m.ch <- fmt.Sprintf(format, args...)
}

// Errorf is implemeted to comply with rig log.Logger interface.
func (m *MessageWriter) Errorf(format string, args ...interface{}) {
	m.ch <- fmt.Sprintf(format, args...)
}

// loop keeps reading messages from the channel and printint them
// using the provided WriteFn. Exits when the channel is closed.
func (m *MessageWriter) loop() {
	var counter int
	var message string
	var ticker = time.NewTicker(50 * time.Millisecond)
	var end bool
	for {
		previous := message

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

		if m.mask != nil {
			message = m.mask(message)
		}

		if m.lbreak != nil && previous != message {
			if lbreak, lcontent := m.lbreak(message); lbreak {
				if diff := len(previous) - len(lcontent); diff > 0 {
					suffix := strings.Repeat(" ", diff)
					lcontent = fmt.Sprintf("%s%s", lcontent, suffix)
				}
				m.printf("\033[K\r✔  %s\n", lcontent)
			}
		}

		pos := counter % len(blocks)
		if !end {
			m.printf("\033[K\r%s  %s", blocks[pos], message)
			continue
		}

		prefix := "✔"
		if m.err {
			prefix = "✗"
		}
		m.printf("\033[K\r%s  %s\n", prefix, message)
		close(m.end)
		return
	}
}

// Start starts a progress bar.
func Start(opts ...Option) *MessageWriter {
	mw := &MessageWriter{
		ch:     make(chan string, 1024),
		end:    make(chan struct{}),
		printf: fmt.Printf,
	}
	for _, opt := range opts {
		opt(mw)
	}
	go mw.loop()
	return mw
}
