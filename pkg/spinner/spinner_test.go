package spinner

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func WriteTo(to io.Writer) WriteFn {
	return func(format string, args ...interface{}) (int, error) {
		fmt.Fprintf(to, format, args...)
		return 0, nil
	}
}

func TestStartAndClosef(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WithWriter(WriteTo(buf)))
	pb.Infof("hello")
	time.Sleep(time.Second)
	pb.Closef("closing with this  value")
	assert.Contains(t, buf.String(), "closing with this  value")
}

func TestStartAndClose(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WithWriter(WriteTo(buf)))
	pb.Infof("hello")
	pb.Close()
	assert.Contains(t, buf.String(), "hello")
}

func TestStartAndWrite(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WithWriter(WriteTo(buf)))
	pb.Infof("hello")
	_, err := pb.Write([]byte("world"))
	assert.NoError(t, err)
	pb.Close()
	assert.Contains(t, buf.String(), "world")
}

func TestStartAndTracef(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WithWriter(WriteTo(buf)))
	pb.Tracef("tracef")
	pb.Close()
	assert.Contains(t, buf.String(), "tracef")
}

func TestStartAndDebugf(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WithWriter(WriteTo(buf)))
	pb.Debugf("debugf")
	pb.Close()
	assert.Contains(t, buf.String(), "debugf")
}

func TestStartAndWarnf(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WithWriter(WriteTo(buf)))
	pb.Warnf("warnf")
	pb.Close()
	assert.Contains(t, buf.String(), "warnf")
}

func TestStartAndErrorf(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WithWriter(WriteTo(buf)))
	pb.Errorf("errorf")
	pb.Close()
	assert.Contains(t, buf.String(), "errorf")
}

func TestStartAndCloseWithError(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WithWriter(WriteTo(buf)))
	for i := 0; i < 1000; i++ {
		pb.Infof("test nr %d", i)
	}
	pb.CloseWithError()
	assert.Contains(t, buf.String(), "✗")
}

func TestMask(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	maskfn := func(s string) string {
		if s == "test 0" {
			return "masked 0"
		}
		if s == "test 1" {
			return "masked 1"
		}
		return s
	}
	pb := Start(
		WithWriter(WriteTo(buf)),
		WithMask(maskfn),
	)
	pb.Infof("test 0")
	pb.Infof("test 1")
	pb.Close()
	assert.Contains(t, buf.String(), "masked 0")
	assert.Contains(t, buf.String(), "masked 1")
}

func TestLineBreak(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	lbreak := func(s string) bool {
		return s == "test 3" || s == "test 8"
	}
	pb := Start(
		WithWriter(WriteTo(buf)),
		WithLineBreaker(lbreak),
	)
	for i := 0; i < 100; i++ {
		pb.Infof("test %d", i)
	}
	pb.Close()
	// we expect the following output:
	// ✔  test 3 (\n)
	// ✔  test 8 (\n)
	// ✔  test 99 (\n)
	assert.Equal(t, strings.Count(buf.String(), "\n"), 3)
	assert.Contains(t, buf.String(), "test 3")
	assert.Contains(t, buf.String(), "test 8")
	assert.Contains(t, buf.String(), "test 99")
}
