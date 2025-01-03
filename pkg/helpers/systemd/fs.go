package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UnitFilePath returns the path to a systemd unit file.
func UnitFilePath(unit string) string {
	unit = normalizeUnitName(unit)
	return filepath.Join("/etc/systemd/system", unit)
}

// WriteUnitFile writes a systemd unit file.
func WriteUnitFile(unit string, contents []byte) error {
	unit = normalizeUnitName(unit)

	err := os.WriteFile(UnitFilePath(unit), contents, 0644)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// DropInFilePath returns the path to a systemd drop-in file.
func DropInFilePath(unit, fileName string) string {
	unit = normalizeUnitName(unit)
	return filepath.Join("/etc/systemd/system", fmt.Sprintf("%s.d", unit), normalizeDropInFileName(fileName))
}

// WriteDropInFile writes a systemd drop-in file to the unit's drop-in directory.
func WriteDropInFile(unit, fileName string, contents []byte) error {
	unit = normalizeUnitName(unit)

	dir := fmt.Sprintf("/etc/systemd/system/%s.d", unit)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	err = os.WriteFile(DropInFilePath(unit, fileName), contents, 0644)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func normalizeDropInFileName(name string) string {
	if !strings.HasSuffix(name, ".conf") {
		name = fmt.Sprintf("%s.conf", name)
	}
	return name
}
