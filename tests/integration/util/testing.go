package util

import (
	"fmt"
	"strings"
	"testing"
)

func GenerateClusterName(t *testing.T) string {
	return fmt.Sprintf("inttest-%s",
		strings.ReplaceAll(
			strings.ToLower(t.Name()),
			"_", "-",
		),
	)
}
