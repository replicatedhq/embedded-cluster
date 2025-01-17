package systemd

import (
	"fmt"
	"strings"
)

func normalizeUnitName(unit string) string {
	if !strings.HasSuffix(unit, ".service") {
		unit = fmt.Sprintf("%s.service", unit)
	}
	return unit
}
