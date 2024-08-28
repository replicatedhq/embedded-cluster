package controllers

import (
	"testing"

	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/require"
)

func Test_detectChartCompletion(t *testing.T) {
	type args struct {
		combinedConfigs *k0sv1beta1.HelmExtensions
		installedCharts k0shelm.ChartList
	}
	tests := []struct {
		name                 string
		args                 args
		wantChartErrors      []string
		wantIncompleteCharts []string
	}{
		{
			name: "no drift",
			args: args{
				combinedConfigs: &k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "test",
							Version: "1.0.0",
						},
						{
							Name:    "test2",
							Version: "2.0.0",
						},
					},
				},
				installedCharts: k0shelm.ChartList{
					Items: []k0shelm.Chart{
						{
							Status: k0shelm.ChartStatus{
								Version:    "1.0.0",
								ValuesHash: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
							},
							Spec: k0shelm.ChartSpec{
								ReleaseName: "test",
							},
						},
						{
							Status: k0shelm.ChartStatus{
								Version:    "2.0.0",
								ValuesHash: "60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c752",
							},
							Spec: k0shelm.ChartSpec{
								ReleaseName: "test2",
							},
						},
					},
				},
			},
			wantIncompleteCharts: []string{},
			wantChartErrors:      []string{},
		},
		{
			name: "new chart",
			args: args{
				combinedConfigs: &k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "test",
							Version: "1.0.0",
						},
						{
							Name:    "test2",
							Version: "2.0.0",
						},
					},
				},
				installedCharts: k0shelm.ChartList{
					Items: []k0shelm.Chart{
						{
							Status: k0shelm.ChartStatus{
								Version:    "1.0.0",
								ValuesHash: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
							},
							Spec: k0shelm.ChartSpec{
								ReleaseName: "test",
							},
						},
					},
				},
			},
			wantIncompleteCharts: []string{"test2"},
			wantChartErrors:      []string{},
		},
		{
			name: "removed chart",
			args: args{
				combinedConfigs: &k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "test",
							Version: "1.0.0",
						},
					},
				},
				installedCharts: k0shelm.ChartList{
					Items: []k0shelm.Chart{
						{
							Status: k0shelm.ChartStatus{
								Version:    "1.0.0",
								ValuesHash: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
							},
							Spec: k0shelm.ChartSpec{
								ReleaseName: "test",
							},
						},
						{
							Status: k0shelm.ChartStatus{
								Version: "2.0.0",
							},
							Spec: k0shelm.ChartSpec{
								ReleaseName: "test2",
							},
						},
					},
				},
			},
			wantIncompleteCharts: []string{},
			wantChartErrors:      []string{},
		},
		{
			name: "added and removed chart",
			args: args{
				combinedConfigs: &k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "test2",
							Version: "2.0.0",
						},
					},
				},
				installedCharts: k0shelm.ChartList{
					Items: []k0shelm.Chart{
						{
							Status: k0shelm.ChartStatus{
								Version:    "1.0.0",
								ValuesHash: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
							},
							Spec: k0shelm.ChartSpec{
								ReleaseName: "test",
							},
						},
					},
				},
			},
			wantIncompleteCharts: []string{"test2"},
			wantChartErrors:      []string{},
		},
		{
			name: "no drift, but error",
			args: args{
				combinedConfigs: &k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "test",
							Version: "1.0.0",
						},
						{
							Name:    "test2",
							Version: "2.0.0",
						},
					},
				},
				installedCharts: k0shelm.ChartList{
					Items: []k0shelm.Chart{
						{
							Status: k0shelm.ChartStatus{
								Error: "test chart error",
							},
							Spec: k0shelm.ChartSpec{
								Version:     "1.0.0",
								ReleaseName: "test",
							},
						},
						{
							Status: k0shelm.ChartStatus{
								Error: "test chart two error",
							},
							Spec: k0shelm.ChartSpec{
								Version:     "2.0.0",
								ReleaseName: "test2",
							},
						},
					},
				},
			},
			wantIncompleteCharts: []string{},
			wantChartErrors:      []string{"test chart error", "test chart two error"},
		},
		{
			name: "drift and error",
			args: args{
				combinedConfigs: &k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "test",
							Version: "1.0.0",
						},
						{
							Name:    "test2",
							Version: "2.0.1",
						},
					},
				},
				installedCharts: k0shelm.ChartList{
					Items: []k0shelm.Chart{
						{
							Status: k0shelm.ChartStatus{
								Error:   "test chart error",
								Version: "1.0.0",
							},
							Spec: k0shelm.ChartSpec{
								ReleaseName: "test",
							},
						},
						{
							Status: k0shelm.ChartStatus{
								Error:   "test chart two error",
								Version: "2.0.0",
							},
							Spec: k0shelm.ChartSpec{
								ReleaseName: "test2",
							},
						},
					},
				},
			},
			wantIncompleteCharts: []string{},
			wantChartErrors:      []string{"test chart error", "test chart two error"},
		},
		{
			name: "drift values",
			args: args{
				combinedConfigs: &k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "test",
							Version: "1.0.0",
							Values: `
                foo: bar
              `,
						},
					},
				},
				installedCharts: k0shelm.ChartList{
					Items: []k0shelm.Chart{
						{
							Status: k0shelm.ChartStatus{
								Version:    "1.0.0",
								ValuesHash: "b87d302884cfcf4e2950a48468fc703baa00a749c552c72eccc1f1914c92e19a",
							},
							Spec: k0shelm.ChartSpec{
								ReleaseName: "test",
								Values: `
                  foo: asdf
                `,
							},
						},
					},
				},
			},
			wantIncompleteCharts: []string{"test"},
			wantChartErrors:      []string{},
		},
		{
			name: "values hash differs",
			args: args{
				combinedConfigs: &k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "test",
							Version: "1.0.0",
							Values: `
                foo: bar
              `,
						},
					},
				},
				installedCharts: k0shelm.ChartList{
					Items: []k0shelm.Chart{
						{
							Status: k0shelm.ChartStatus{
								Version:    "1.0.0",
								ValuesHash: "incorrect hash",
							},
							Spec: k0shelm.ChartSpec{
								ReleaseName: "test",
								Values: `
                  foo: bar
                `,
							},
						},
					},
				},
			},
			wantIncompleteCharts: []string{"test"},
			wantChartErrors:      []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			gotIncomplete, gotErrors, err := detectChartCompletion(tt.args.combinedConfigs, tt.args.installedCharts)
			req.NoError(err)
			req.Equal(tt.wantChartErrors, gotErrors)
			req.Equal(tt.wantIncompleteCharts, gotIncomplete)
		})
	}
}

func Test_detectChartDrift2(t *testing.T) {
	tests := []struct {
		name         string
		configCharts *k0sv1beta1.HelmExtensions
		charts       k0shelm.ChartList
		want         []string
	}{
		{
			name:         "no charts",
			configCharts: nil,
			want:         []string{},
		},
		{
			name:         "no config charts",
			configCharts: &k0sv1beta1.HelmExtensions{},
			want:         []string{},
		},
		{
			name: "all charts present",
			configCharts: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name: "test",
					},
					{
						Name: "test2",
					},
				},
			},
			charts: k0shelm.ChartList{
				Items: []k0shelm.Chart{
					{
						Spec: k0shelm.ChartSpec{
							ReleaseName: "test",
						},
						Status: k0shelm.ChartStatus{
							ReleaseName: "test",
							ValuesHash:  "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
						},
					},
					{
						Spec: k0shelm.ChartSpec{
							ReleaseName: "test2",
						},
						Status: k0shelm.ChartStatus{
							ReleaseName: "test2",
							ValuesHash:  "60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c752",
						},
					},
				},
			},
			want: []string{},
		},
		{
			name: "one chart not present in cluster",
			configCharts: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name: "test",
					},
					{
						Name: "test2",
					},
				},
			},
			charts: k0shelm.ChartList{
				Items: []k0shelm.Chart{
					{
						Spec: k0shelm.ChartSpec{
							ReleaseName: "test2",
						},
						Status: k0shelm.ChartStatus{
							ReleaseName: "test2",
							ValuesHash:  "60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c752",
						},
					},
				},
			},
			want: []string{"test"},
		},
		{
			name: "chart present but not yet applied",
			configCharts: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1",
					},
				},
			},
			charts: k0shelm.ChartList{
				Items: []k0shelm.Chart{
					{
						Spec: k0shelm.ChartSpec{
							ReleaseName: "test",
						},
						Status: k0shelm.ChartStatus{},
					},
				},
			},
			want: []string{"test"},
		},
		{
			name: "one chart ok, one not applied, one not present, one applying",
			configCharts: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "okchart",
						Version: "1",
					},
					{
						Name:    "notapplied",
						Version: "1",
					},
					{
						Name:    "notpresent",
						Version: "1",
					},
					{
						Name:    "applying",
						Version: "2",
					},
				},
			},
			charts: k0shelm.ChartList{
				Items: []k0shelm.Chart{
					{
						Spec: k0shelm.ChartSpec{
							ReleaseName: "okchart",
						},
						Status: k0shelm.ChartStatus{
							ReleaseName: "okchart",
							Version:     "1",
							ValuesHash:  "c21bd877b996eed13c65080deab39ef6bec3fe475a39bf506f5afd095ba2aa95",
						},
					},
					{
						Spec: k0shelm.ChartSpec{
							ReleaseName: "notapplied",
						},
						Status: k0shelm.ChartStatus{},
					},
					{
						Spec: k0shelm.ChartSpec{
							ReleaseName: "applying",
						},
						Status: k0shelm.ChartStatus{
							ReleaseName: "applying",
							Version:     "2",
							ValuesHash:  "incorrect hash",
						},
					},
				},
			},
			want: []string{"notapplied", "notpresent", "applying"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			got, charterrs, err := detectChartCompletion(tt.configCharts, tt.charts)
			req.NoError(err)
			req.Empty(charterrs)
			req.ElementsMatch(tt.want, got)
		})
	}
}

func Test_detectChartDrift(t *testing.T) {
	tests := []struct {
		name            string
		combinedConfigs *k0sv1beta1.HelmExtensions
		currentConfigs  *k0sv1beta1.HelmExtensions
		want            bool
		wantNames       []string
	}{
		{
			name: "one chart no drift",
			combinedConfigs: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz # with a comment",
					},
				},
			},
			currentConfigs: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
			want:      false,
			wantNames: []string{},
		},
		{
			name: "one chart different values",
			combinedConfigs: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: newvalue",
					},
				},
			},
			currentConfigs: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
			want:      true,
			wantNames: []string{"test"},
		},
		{
			name: "one chart different version",
			combinedConfigs: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.1",
						Values:  "abc: xyz",
					},
				},
			},
			currentConfigs: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
			want:      true,
			wantNames: []string{"test"},
		},
		{
			name: "new chart added",
			combinedConfigs: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
					{
						Name:    "newchart",
						Version: "2.0.0",
					},
				},
			},
			currentConfigs: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
			want:      true,
			wantNames: []string{"newchart"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			got, gotNames, err := detectChartDrift(tt.combinedConfigs, tt.currentConfigs)
			req.NoError(err)
			req.Equal(tt.want, got)
			req.Equal(tt.wantNames, gotNames)
		})
	}
}

func Test_yamlDiff(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{
			name: "comments",
			a:    "abc: xyz",
			b:    "abc: xyz # with a comment",
			want: false,
		},
		{
			name: "order",
			a:    "abc: xyz\nkey2: val2",
			b:    "key2: val2\nabc: xyz",
			want: false,
		},
		{
			name: "different values",
			a:    "abc: xyz",
			b:    "abc: newvalue",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			got, err := yamlDiff(tt.a, tt.b)
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}
