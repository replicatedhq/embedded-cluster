package progressbar

import (
	"bytes"
	"fmt"
	"io"
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
	pb := Start(WriteTo(buf))
	pb.Infof("hello")
	time.Sleep(time.Second)
	pb.Closef("closing with this  value")
	assert.Contains(t, buf.String(), "closing with this  value")
}

func TestStartAndClose(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WriteTo(buf))
	pb.Infof("hello")
	pb.Close()
	assert.Contains(t, buf.String(), "hello")
}

func TestStartAndWrite(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WriteTo(buf))
	pb.Infof("hello")
	_, err := pb.Write([]byte("world"))
	assert.NoError(t, err)
	pb.Close()
	assert.Contains(t, buf.String(), "world")
}

func TestStartAndTracef(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WriteTo(buf))
	pb.Tracef("tracef")
	pb.Close()
	assert.Contains(t, buf.String(), "tracef")
}

func TestStartAndDebugf(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WriteTo(buf))
	pb.Debugf("debugf")
	pb.Close()
	assert.Contains(t, buf.String(), "debugf")
}

func TestStartAndWarnf(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WriteTo(buf))
	pb.Warnf("warnf")
	pb.Close()
	assert.Contains(t, buf.String(), "warnf")
}

func TestStartAndErrorf(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	pb := Start(WriteTo(buf))
	pb.Errorf("errorf")
	pb.Close()
	assert.Contains(t, buf.String(), "errorf")
}
