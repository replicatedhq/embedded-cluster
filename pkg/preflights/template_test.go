// Package preflights manages running host preflights on remote hosts.
package preflights

import (
	"context"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	"github.com/stretchr/testify/require"
)

func getSubnetCollectorByName(name string, spec v1beta2.HostPreflightSpec) *v1beta2.SubnetAvailable {
	for _, c := range spec.Collectors {
		if c.SubnetAvailable == nil {
			continue
		}
		if c.SubnetAvailable.CollectorName == name {
			return c.SubnetAvailable
		}
	}
	return nil
}

func TestTemplateWithCIDRData(t *testing.T) {
	tests := []struct {
		name             string
		podCIDR          string
		serviceCIDR      string
		globalCIDR       string
		wantErr          bool
		expectCollectors []v1beta2.SubnetAvailable
		expectAnalyzers  []v1beta2.SubnetAvailableAnalyze
	}{
		{
			name:    "valid podCIDR",
			podCIDR: "10.0.0.0/24",
			expectCollectors: []v1beta2.SubnetAvailable{
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Pods Subnet",
						Exclude:       multitype.FromString("false"),
					},
					CIDRRangeAlloc: "10.0.0.0/24",
					DesiredCIDR:    24,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			tl, err := TemplateData{}.WithCIDRData(test.podCIDR, test.serviceCIDR, test.globalCIDR)
			if test.wantErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}
			hpfc, err := GetClusterHostPreflights(context.Background(), tl)
			req.NoError(err)

			spec := hpfc[0].Spec

			for _, collector := range test.expectCollectors {
				actual := getSubnetCollectorByName(collector.CollectorName, spec)
				req.NotNil(actual)
				req.Equal(collector, *actual)
			}
		})
	}
}
