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

//go:embed host-preflight/resolv-conf.yaml
var resolvConfHostPreflightYAML string

func GetClusterHostPreflights(ctx context.Context, data types.HostPreflightTemplateData) ([]v1beta2.HostPreflight, error) {
	spec, err := renderHostPreflightTemplate(clusterHostPreflightYAML, data)
	if err != nil {
		return nil, fmt.Errorf("render host preflight template: %w", err)
	}

	resolvConfSpec, err := renderHostPreflightTemplate(resolvConfHostPreflightYAML, data)
	if err != nil {
		return nil, fmt.Errorf("render resolv conf host preflight template: %w", err)
	}

	kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{
		RawSpecs: []string{
			spec,
			resolvConfSpec,
		},
		Strict: true,
	})
	if err != nil {
		return nil, fmt.Errorf("load host preflight specs: %w", err)
	}
	return kinds.HostPreflightsV1Beta2, nil
}

func renderHostPreflightTemplate(spec string, data types.HostPreflightTemplateData) (string, error) {
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

type AirgapStorageSpaceCalcArgs struct { // Use struct instead of positional args not to accidentally pass in wrong int64 arg
	UncompressedSize   int64
	EmbeddedAssetsSize int64
	K0sImageSize       int64
	IsController       bool
}

// CalculateAirgapStorageSpace calculates required storage space for airgap installations.
// Controller nodes need 2x uncompressed size, worker nodes need 1x ec infra image size. Returns "XGi" or "XMi".
func CalculateAirgapStorageSpace(data AirgapStorageSpaceCalcArgs) string {
	requiredBytes := data.K0sImageSize
	if data.IsController {
		// Controller nodes require 2x the extracted bundle size for processing
		requiredBytes = data.UncompressedSize * 2
	}

	requiredBytes += data.EmbeddedAssetsSize

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
