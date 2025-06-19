package preflights

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"math"
	"text/template"

	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights/types"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
)

//go:embed host-preflight.yaml
var clusterHostPreflightYAML string

func GetClusterHostPreflights(ctx context.Context, data types.TemplateData) ([]v1beta2.HostPreflight, error) {
	spec, err := renderTemplate(clusterHostPreflightYAML, data)
	if err != nil {
		return nil, fmt.Errorf("render host preflight template: %w", err)
	}
	kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{
		RawSpecs: []string{
			spec,
		},
		Strict: true,
	})
	if err != nil {
		return nil, fmt.Errorf("load host preflight specs: %w", err)
	}
	return kinds.HostPreflightsV1Beta2, nil
}

func renderTemplate(spec string, data types.TemplateData) (string, error) {
	tmpl, err := template.New("preflight").Parse(spec)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	buf := bytes.NewBuffer(nil)
	err = tmpl.Execute(buf, data)
	if err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// CalculateControllerAirgapStorageSpace calculates the required airgap storage space for controller nodes.
// It multiplies the uncompressed size by 2, rounds up to the nearest natural number, and returns a string.
// The quantity will be in Gi for sizes >= 1 Gi, or Mi for smaller sizes.
func CalculateControllerAirgapStorageSpace(uncompressedSize int64) string {
	if uncompressedSize <= 0 {
		return ""
	}

	// Controller nodes require 2x the extracted bundle size for processing
	requiredBytes := uncompressedSize * 2

	// Convert to Gi if >= 1 Gi, otherwise use Mi
	if requiredBytes >= 1024*1024*1024 { // 1 Gi in bytes
		// Convert to Gi and round up to nearest natural number
		giValue := float64(requiredBytes) / (1024 * 1024 * 1024)
		roundedGi := math.Ceil(giValue)
		return fmt.Sprintf("%dGi", int64(roundedGi))
	} else {
		// Convert to Mi and round up to nearest natural number
		miValue := float64(requiredBytes) / (1024 * 1024)
		roundedMi := math.Ceil(miValue)
		return fmt.Sprintf("%dMi", int64(roundedMi))
	}
}
