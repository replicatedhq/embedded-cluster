package dryrun

import (
	_ "embed"
	"testing"
	"time"
)

func TestJoin(t *testing.T) {
	dryrunJoin(t, "192.168.10.1:30000", "some-token")
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
