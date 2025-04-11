package spinner

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func startTest(opts ...Option) (*MessageWriter, *bytes.Buffer) {
	buf := bytes.NewBuffer(nil)
	opts = append(
		[]Option{
			WithWriter(writeTo(buf)),
			func(m *MessageWriter) {
				m.tty = true
			},
		},
		opts...,
	)
	pb := Start(opts...)
	return pb, buf
}

func writeTo(to io.Writer) WriteFn {
	return func(format string, args ...interface{}) (int, error) {
		fmt.Fprintf(to, format, args...)
		return 0, nil
	}
}

func TestStartAndClosef(t *testing.T) {
	pb, buf := startTest()
	pb.Infof("hello")
	time.Sleep(time.Second)
	pb.Closef("closing with this  value")
	assert.Contains(t, buf.String(), "closing with this  value")
	assert.Contains(t, buf.String(), "✔") // Should contain success symbol
}

func TestStartAndErrorClosef(t *testing.T) {
	pb, buf := startTest()
	pb.Infof("hello")
	pb.ErrorClosef("error occurred: %s", "something went wrong")
	assert.Contains(t, buf.String(), "error occurred: something went wrong")
	assert.Contains(t, buf.String(), "✗") // Should contain error symbol
}

func TestStartAndClose(t *testing.T) {
	pb, buf := startTest()
	pb.Infof("hello")
	pb.Close()
	assert.Contains(t, buf.String(), "hello")
}

func TestStartAndWrite(t *testing.T) {
	pb, buf := startTest()
	pb.Infof("hello")
	_, err := pb.Write([]byte("world"))
	assert.NoError(t, err)
	pb.Close()
	assert.Contains(t, buf.String(), "world")
}

func TestStartAndTracef(t *testing.T) {
	pb, buf := startTest()
	pb.Tracef("tracef")
	pb.Close()
	assert.Contains(t, buf.String(), "tracef")
}

func TestStartAndDebugf(t *testing.T) {
	pb, buf := startTest()
	pb.Debugf("debugf")
	pb.Close()
	assert.Contains(t, buf.String(), "debugf")
}

func TestStartAndWarnf(t *testing.T) {
	pb, buf := startTest()
	pb.Warnf("warnf")
	pb.Close()
	assert.Contains(t, buf.String(), "warnf")
}

func TestStartAndErrorf(t *testing.T) {
	pb, buf := startTest()
	pb.Errorf("errorf")
	pb.Close()
	assert.Contains(t, buf.String(), "errorf")
}

func TestStartAndCloseWithError(t *testing.T) {
	pb, buf := startTest()
	for i := 0; i < 1000; i++ {
		pb.Infof("test nr %d", i)
	}
	pb.CloseWithError()
	assert.Contains(t, buf.String(), "✗")
}

func TestMask(t *testing.T) {
	maskfn := func(s string) string {
		if s == "test 0" {
			return "masked 0"
		}
		if s == "test 1" {
			return "masked 1"
		}
		return s
	}
	pb, buf := startTest(
		WithMask(maskfn),
	)
	pb.Infof("test 0")
	pb.Infof("test 1")
	pb.Close()
	assert.Contains(t, buf.String(), "masked 0")
	assert.Contains(t, buf.String(), "masked 1")
}

func TestNoTTY(t *testing.T) {
	pb, buf := startTest(
		func(m *MessageWriter) {
			m.tty = false
		},
	)

	pb.Infof("Installing")
	time.Sleep(time.Second)
	pb.Infof("Waiting")
	time.Sleep(time.Second)
	pb.Infof("Done")
	pb.Close()

	assert.Equal(t, "○  Installing\n○  Waiting\n○  Done\n✔  Done\n", buf.String())
}
