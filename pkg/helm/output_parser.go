package helm

import (
	"regexp"
	"strings"
)

var separator = regexp.MustCompile(`(?:^|\n)\s*---\s*(?:\n|$)`)

// splitManifests parses multi-doc YAML manifests and returns them as byte slices
func splitManifests(yamlOutput string) ([][]byte, error) {
	result := [][]byte{}

	// Make sure that any extra whitespace in YAML stream doesn't interfere in splitting documents correctly.
	manifests := separator.Split(strings.TrimSpace(yamlOutput), -1)

	for _, manifest := range manifests {
		manifest = strings.TrimSpace(manifest)
		if manifest == "" {
			continue
		}
		result = append(result, []byte(manifest))
	}

	return result, nil
}
