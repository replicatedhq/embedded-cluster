package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
)

const (
	DefaultSystemDUnitBasePath = "/etc/systemd/system"
)

var (
	_systemDUnitBasePath = DefaultSystemDUnitBasePath
)

// SetSystemDUnitBasePath sets the base path for systemd unit files.
func SetSystemDUnitBasePath(path string) {
	_systemDUnitBasePath = path
}

// UnitFilePath returns the path to a systemd unit file.
func UnitFilePath(unit string) string {
	unit = normalizeUnitName(unit)
	return filepath.Join(_systemDUnitBasePath, unit)
}

// WriteUnitFile writes a systemd unit file.
func WriteUnitFile(unit string, contents []byte) error {
	unit = normalizeUnitName(unit)

	err := helpers.WriteFile(UnitFilePath(unit), contents, 0644)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// DropInDirPath returns the path to a systemd unit drop-in directory.
func DropInDirPath(unit string) string {
	unit = normalizeUnitName(unit)
	return filepath.Join(_systemDUnitBasePath, fmt.Sprintf("%s.d", unit))
}

// DropInFilePath returns the path to a systemd drop-in file.
func DropInFilePath(unit, fileName string) string {
	return filepath.Join(DropInDirPath(unit), normalizeDropInFileName(fileName))
}

// WriteDropInFile writes a systemd drop-in file to the unit's drop-in directory.
func WriteDropInFile(unit, fileName string, contents []byte) error {
	unit = normalizeUnitName(unit)

	dir := filepath.Join(_systemDUnitBasePath, fmt.Sprintf("%s.d", unit))
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	err = helpers.WriteFile(DropInFilePath(unit, fileName), contents, 0644)
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
