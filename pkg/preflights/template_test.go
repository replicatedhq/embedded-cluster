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

func getSubnetAnalyzerByName(name string, spec v1beta2.HostPreflightSpec) *v1beta2.SubnetAvailableAnalyze {
	for _, c := range spec.Analyzers {
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
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Services Subnet",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Subnet",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
			},
			expectAnalyzers: []v1beta2.SubnetAvailableAnalyze{
				{
					CollectorName: "Pods Subnet",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("false"),
					},
				},
				{
					CollectorName: "Services Subnet",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("true"),
					},
				},
				{
					CollectorName: "Subnet",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("true"),
					},
				},
			},
		},
		{
			name:    "not a valid podCIDR",
			podCIDR: "not-a-cidr",
			wantErr: true,
		},
		{
			name:        "valid serviceCIDR",
			serviceCIDR: "10.0.0.0/24",
			expectCollectors: []v1beta2.SubnetAvailable{
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Pods Subnet",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Services Subnet",
						Exclude:       multitype.FromString("false"),
					},
					CIDRRangeAlloc: "10.0.0.0/24",
					DesiredCIDR:    24,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Subnet",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
			},
			expectAnalyzers: []v1beta2.SubnetAvailableAnalyze{
				{
					CollectorName: "Pods Subnet",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("true"),
					},
				},
				{
					CollectorName: "Services Subnet",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("false"),
					},
				},
				{
					CollectorName: "Subnet",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("true"),
					},
				},
			},
		},
		{
			name:        "not a valid serviceCIDR",
			serviceCIDR: "not-a-cidr",
			wantErr:     true,
		},
		{
			name:       "valid CIDR",
			globalCIDR: "10.0.0.0/24",
			expectCollectors: []v1beta2.SubnetAvailable{
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Pods Subnet",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Services Subnet",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Subnet",
						Exclude:       multitype.FromString("false"),
					},
					CIDRRangeAlloc: "10.0.0.0/24",
					DesiredCIDR:    24,
				},
			},
			expectAnalyzers: []v1beta2.SubnetAvailableAnalyze{
				{
					CollectorName: "Pods Subnet",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("true"),
					},
				},
				{
					CollectorName: "Services Subnet",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("true"),
					},
				},
				{
					CollectorName: "Subnet",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("false"),
					},
				},
			},
		},
		{
			name:       "not a valid CIDR",
			globalCIDR: "not-a-cidr",
			wantErr:    true,
		},
		{
			name:        "valid podCIDR and serviceCIDR are provided",
			podCIDR:     "10.1.0.0/24",
			serviceCIDR: "10.2.0.0/24",
			globalCIDR:  "",
			expectCollectors: []v1beta2.SubnetAvailable{
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Pods Subnet",
						Exclude:       multitype.FromString("false"),
					},
					CIDRRangeAlloc: "10.1.0.0/24",
					DesiredCIDR:    24,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Services Subnet",
						Exclude:       multitype.FromString("false"),
					},
					CIDRRangeAlloc: "10.2.0.0/24",
					DesiredCIDR:    24,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Subnet",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
			},
			expectAnalyzers: []v1beta2.SubnetAvailableAnalyze{
				{
					CollectorName: "Pods Subnet",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("false"),
					},
				},
				{
					CollectorName: "Services Subnet",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("false"),
					},
				},
				{
					CollectorName: "Subnet",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("true"),
					},
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

			for _, analyzer := range test.expectAnalyzers {
				actual := getSubnetAnalyzerByName(analyzer.CollectorName, spec)
				req.NotNil(actual)
				req.Equal(analyzer.Exclude, actual.Exclude)
			}
		})
	}
}
