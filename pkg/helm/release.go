package helm

import (
	"encoding/json"
	"fmt"
)

// ReleaseInfo holds the result of a Helm install or upgrade operation.
// It replaces *release.Release from the helm SDK to decouple the Client
// interface from the Helm SDK version.
type ReleaseInfo struct {
	Name      string
	Namespace string
	Status    string
	Revision  int
	Chart     string
	Version   string
}

// helmReleaseJSON is the JSON structure returned by helm install/upgrade --output json.
type helmReleaseJSON struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   int    `json:"version"`
	Info      struct {
		Status string `json:"status"`
	} `json:"info"`
	Chart struct {
		Metadata struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"metadata"`
	} `json:"chart"`
}

func parseReleaseOutput(stdout string) (*ReleaseInfo, error) {
	var out helmReleaseJSON
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		return nil, fmt.Errorf("parse release JSON: %w", err)
	}
	return &ReleaseInfo{
		Name:      out.Name,
		Namespace: out.Namespace,
		Status:    out.Info.Status,
		Revision:  out.Version,
		Chart:     out.Chart.Metadata.Name,
		Version:   out.Chart.Metadata.Version,
	}, nil
}
