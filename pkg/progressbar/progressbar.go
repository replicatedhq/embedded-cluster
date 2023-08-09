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

// MessageWriter implements io.Writer on top of a channel of strings.
type MessageWriter chan string

// Write implements io.Writer for the MessageWriter.
func (m MessageWriter) Write(p []byte) (n int, err error) {
	str := string(p)
	lines := strings.Split(str, "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		m <- line
	}
	return len(p), nil
}

// Close closes the MessageWriter inner channel.
func (m MessageWriter) Close() {
	close(m)
}

// Tracef is implemeted to comply with rig log.Logger interface.
func (m MessageWriter) Tracef(format string, args ...interface{}) {
	m <- fmt.Sprintf(format, args...)
}

// Debugf is implemeted to comply with rig log.Logger interface.
func (m MessageWriter) Debugf(format string, args ...interface{}) {
	m <- fmt.Sprintf(format, args...)
}

// Infof is implemeted to comply with rig log.Logger interface.
func (m MessageWriter) Infof(format string, args ...interface{}) {
	m <- fmt.Sprintf(format, args...)
}

// Warnf is implemeted to comply with rig log.Logger interface.
func (m MessageWriter) Warnf(format string, args ...interface{}) {
	m <- fmt.Sprintf(format, args...)
}

// Errorf is implemeted to comply with rig log.Logger interface.
func (m MessageWriter) Errorf(format string, args ...interface{}) {
	m <- fmt.Sprintf(format, args...)
}

// Start starts the progress bar. Returns a writer the caller can use to
// send us messages to be printed on the screen and a channel to indicate
// that the progress bar has finished its work (channel is closed at end)
// The progress bar will keep running until the MessageWriter is closed.
func Start() (MessageWriter, chan struct{}) {
	finished := make(chan struct{})
	mwriter := make(chan string, 1024)
	go func() {
		var counter int
		var message string
		var ticker = time.NewTicker(50 * time.Millisecond)
		var end bool
		for {
			select {
			case msg, open := <-mwriter:
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
				fmt.Printf("\033[K\r%s  %s ", blocks[pos], message)
				continue
			}
			fmt.Printf("\033[K\r✓  %s\n", message)
			close(finished)
			return
		}
	}()
	return mwriter, finished
}
