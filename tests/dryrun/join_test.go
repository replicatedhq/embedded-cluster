package dryrun

import (
	_ "embed"
	"testing"
	"time"
)

func TestJoin(t *testing.T) {
	dryrunJoin(t, "10.0.0.1", "some-token")
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
