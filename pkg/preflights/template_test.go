// Package preflights manages running host preflights on remote hosts.
package preflights

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/preflights/types"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
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
		globalCIDR       *string
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
						CollectorName: "Pod CIDR",
						Exclude:       multitype.FromString("false"),
					},
					CIDRRangeAlloc: "10.0.0.0/24",
					DesiredCIDR:    24,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Service CIDR",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "CIDR",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
			},
			expectAnalyzers: []v1beta2.SubnetAvailableAnalyze{
				{
					CollectorName: "Pod CIDR",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("false"),
					},
					Outcomes: []*v1beta2.Outcome{
						{
							Fail: &v1beta2.SingleOutcome{
								When:    "no-subnet-available",
								Message: "10.0.0.0/24 is not available. Use --pod-cidr to specify an available CIDR block.",
							},
						},
						{
							Pass: &v1beta2.SingleOutcome{
								When:    "a-subnet-is-available",
								Message: "Specified Pod CIDR is available.",
							},
						},
					},
				},
				{
					CollectorName: "Service CIDR",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("true"),
					},
				},
				{
					CollectorName: "CIDR",
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
						CollectorName: "Pod CIDR",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Service CIDR",
						Exclude:       multitype.FromString("false"),
					},
					CIDRRangeAlloc: "10.0.0.0/24",
					DesiredCIDR:    24,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "CIDR",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
			},
			expectAnalyzers: []v1beta2.SubnetAvailableAnalyze{
				{
					CollectorName: "Pod CIDR",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("true"),
					},
				},
				{
					CollectorName: "Service CIDR",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("false"),
					},
					Outcomes: []*v1beta2.Outcome{
						{
							Fail: &v1beta2.SingleOutcome{
								When:    "no-subnet-available",
								Message: "10.0.0.0/24 is not available. Use --service-cidr to specify an available CIDR block.",
							},
						},
						{
							Pass: &v1beta2.SingleOutcome{
								When:    "a-subnet-is-available",
								Message: "Specified Service CIDR is available.",
							},
						},
					},
				},
				{
					CollectorName: "CIDR",
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
			globalCIDR: ptr.To("10.0.0.0/24"),
			expectCollectors: []v1beta2.SubnetAvailable{
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Pod CIDR",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Service CIDR",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "CIDR",
						Exclude:       multitype.FromString("false"),
					},
					CIDRRangeAlloc: "10.0.0.0/24",
					DesiredCIDR:    24,
				},
			},
			expectAnalyzers: []v1beta2.SubnetAvailableAnalyze{
				{
					CollectorName: "Pod CIDR",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("true"),
					},
				},
				{
					CollectorName: "Service CIDR",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("true"),
					},
				},
				{
					CollectorName: "CIDR",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("false"),
					},
					Outcomes: []*v1beta2.Outcome{
						{
							Fail: &v1beta2.SingleOutcome{
								When:    "no-subnet-available",
								Message: "10.0.0.0/24 is not available. Use --cidr to specify a CIDR block of available private IP addresses (/16 or larger).",
							},
						},
						{
							Pass: &v1beta2.SingleOutcome{
								When:    "a-subnet-is-available",
								Message: "Specified CIDR is available.",
							},
						},
					},
				},
			},
		},
		{
			name:       "not a valid CIDR",
			globalCIDR: ptr.To("not-a-cidr"),
			wantErr:    true,
		},
		{
			name:        "valid podCIDR and serviceCIDR are provided",
			podCIDR:     "10.1.0.0/24",
			serviceCIDR: "10.2.0.0/24",
			globalCIDR:  ptr.To(""),
			expectCollectors: []v1beta2.SubnetAvailable{
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Pod CIDR",
						Exclude:       multitype.FromString("false"),
					},
					CIDRRangeAlloc: "10.1.0.0/24",
					DesiredCIDR:    24,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "Service CIDR",
						Exclude:       multitype.FromString("false"),
					},
					CIDRRangeAlloc: "10.2.0.0/24",
					DesiredCIDR:    24,
				},
				{
					HostCollectorMeta: v1beta2.HostCollectorMeta{
						CollectorName: "CIDR",
						Exclude:       multitype.FromString("true"),
					},
					CIDRRangeAlloc: "",
					DesiredCIDR:    0,
				},
			},
			expectAnalyzers: []v1beta2.SubnetAvailableAnalyze{
				{
					CollectorName: "Pod CIDR",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("false"),
					},
				},
				{
					CollectorName: "Service CIDR",
					AnalyzeMeta: v1beta2.AnalyzeMeta{
						Exclude: multitype.FromString("false"),
					},
				},
				{
					CollectorName: "CIDR",
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
			tl, err := types.TemplateData{}.WithCIDRData(test.podCIDR, test.serviceCIDR, test.globalCIDR)
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
				for _, out := range analyzer.Outcomes {
					req.Contains(actual.Outcomes, out)
				}
			}
		})
	}
}
