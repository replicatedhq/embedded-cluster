package controllers

import (
	"testing"

	"github.com/k0sproject/dig"
	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/release"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func Test_mergeValues(t *testing.T) {
	tests := []struct {
		name            string
		oldValues       string
		newValues       string
		protectedValues []string
		want            string
	}{
		{
			name: "combined test",
			oldValues: `
  password: "foo"
  someField: "asdf"
  other: "text"
  overridden: "abcxyz"
  nested:
    nested:
       protect: "testval"
`,
			newValues: `
  someField: "newstring"
  other: "text"
  overridden: "this is new"
  nested:
    nested:
      newkey: "newval"
      protect: "newval"
`,
			protectedValues: []string{"password", "overridden", "nested.nested.protect"},
			want: `
  password: "foo"
  someField: "newstring"
  nested:
    nested:
      newkey: "newval"
      protect: "testval"
  other: "text"
  overridden: "abcxyz"
`,
		},
		{
			name:      "empty old values",
			oldValues: ``,
			newValues: `
  someField: "newstring"
  other: "text"
  overridden: "this is new"
  nested:
    nested:
      newkey: "newval"
      protect: "newval"
`,
			protectedValues: []string{"password", "overridden", "nested.nested.protect"},
			want: `
  someField: "newstring"
  overridden: "this is new"
  nested:
    nested:
      newkey: "newval"
      protect: "newval"
  other: "text"
`,
		},
		{
			name: "no protected values",
			oldValues: `
	  password: "foo"
	  someField: "asdf"
`,
			newValues: `
password: "newpassword"
`,
			protectedValues: []string{},
			want: `
password: "newpassword"
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			got, err := MergeValues(tt.oldValues, tt.newValues, tt.protectedValues)
			req.NoError(err)

			targetDataMap := dig.Mapping{}
			err = yaml.Unmarshal([]byte(tt.want), &targetDataMap)
			req.NoError(err)

			mergedDataMap := dig.Mapping{}
			err = yaml.Unmarshal([]byte(got), &mergedDataMap)
			req.NoError(err)

			req.Equal(targetDataMap, mergedDataMap)
		})
	}
}

func Test_mergeHelmConfigs(t *testing.T) {
	type args struct {
		meta *release.Meta
		in   v1beta1.Extensions
	}
	tests := []struct {
		name string
		args args
		want *k0sv1beta1.HelmExtensions
	}{
		{
			name: "no meta",
			args: args{
				meta: nil,
				in: v1beta1.Extensions{
					Helm: &k0sv1beta1.HelmExtensions{
						ConcurrencyLevel: 2,
						Repositories:     nil,
						Charts: []k0sv1beta1.Chart{
							{
								Name:    "test",
								Version: "1.0.0",
							},
						},
					},
				},
			},
			want: &k0sv1beta1.HelmExtensions{
				ConcurrencyLevel: 1,
				Repositories:     nil,
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
					},
				},
			},
		},
		{
			name: "add new chart + repo",
			args: args{
				meta: &release.Meta{
					Configs: &k0sv1beta1.HelmExtensions{
						ConcurrencyLevel: 1,
						Repositories: []k0sv1beta1.Repository{
							{
								Name: "origrepo",
							},
						},
						Charts: []k0sv1beta1.Chart{
							{
								Name: "origchart",
							},
						},
					},
				},
				in: v1beta1.Extensions{
					Helm: &k0sv1beta1.HelmExtensions{
						Repositories: []k0sv1beta1.Repository{
							{
								Name: "newrepo",
							},
						},
						Charts: []k0sv1beta1.Chart{
							{
								Name:    "newchart",
								Version: "1.0.0",
							},
						},
					},
				},
			},
			want: &k0sv1beta1.HelmExtensions{
				ConcurrencyLevel: 1,
				Repositories: []k0sv1beta1.Repository{
					{
						Name: "origrepo",
					},
					{
						Name: "newrepo",
					},
				},
				Charts: []k0sv1beta1.Chart{
					{
						Name: "origchart",
					},
					{
						Name:    "newchart",
						Version: "1.0.0",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installation := v1beta1.Installation{
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version:    "1.0.0",
						Extensions: tt.args.in,
					},
				},
			}

			req := require.New(t)
			got := mergeHelmConfigs(tt.args.meta, &installation)
			req.Equal(tt.want, got)
		})
	}
}

func Test_detectChartDrift(t *testing.T) {
	type args struct {
		combinedConfigs *k0sv1beta1.HelmExtensions
		installedCharts k0shelm.ChartList
	}
	tests := []struct {
		name            string
		args            args
		wantChartErrors []string
		wantDrift       bool
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
								ReleaseName: "test",
							},
							Spec: k0shelm.ChartSpec{
								Version: "1.0.0",
							},
						},
						{
							Status: k0shelm.ChartStatus{
								ReleaseName: "test2",
							},
							Spec: k0shelm.ChartSpec{
								Version: "2.0.0",
							},
						},
					},
				},
			},
			wantDrift:       false,
			wantChartErrors: []string{},
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
								ReleaseName: "test",
							},
							Spec: k0shelm.ChartSpec{
								Version: "1.0.0",
							},
						},
					},
				},
			},
			wantDrift:       true,
			wantChartErrors: []string{},
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
								ReleaseName: "test",
							},
							Spec: k0shelm.ChartSpec{
								Version: "1.0.0",
							},
						},
						{
							Status: k0shelm.ChartStatus{
								ReleaseName: "test2",
							},
							Spec: k0shelm.ChartSpec{
								Version: "2.0.0",
							},
						},
					},
				},
			},
			wantDrift:       true,
			wantChartErrors: []string{},
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
								ReleaseName: "test",
							},
							Spec: k0shelm.ChartSpec{
								Version: "1.0.0",
							},
						},
					},
				},
			},
			wantDrift:       true,
			wantChartErrors: []string{},
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
								ReleaseName: "test",
								Error:       "test chart error",
							},
							Spec: k0shelm.ChartSpec{
								Version: "1.0.0",
							},
						},
						{
							Status: k0shelm.ChartStatus{
								ReleaseName: "test2",
								Error:       "test chart two error",
							},
							Spec: k0shelm.ChartSpec{
								Version: "2.0.0",
							},
						},
					},
				},
			},
			wantDrift:       false,
			wantChartErrors: []string{"test chart error", "test chart two error"},
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
								ReleaseName: "test",
								Error:       "test chart error",
							},
							Spec: k0shelm.ChartSpec{
								Version: "1.0.0",
							},
						},
						{
							Status: k0shelm.ChartStatus{
								ReleaseName: "test2",
								Error:       "test chart two error",
							},
							Spec: k0shelm.ChartSpec{
								Version: "2.0.0",
							},
						},
					},
				},
			},
			wantDrift:       true,
			wantChartErrors: []string{"test chart error", "test chart two error"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			gotErrors, gotDrift := detectChartDrift(tt.args.combinedConfigs, tt.args.installedCharts)
			req.Equal(tt.wantChartErrors, gotErrors)
			req.Equal(tt.wantDrift, gotDrift)
		})
	}
}

func Test_generateDesiredCharts(t *testing.T) {
	type args struct {
		meta            *release.Meta
		clusterconfig   k0sv1beta1.ClusterConfig
		combinedConfigs *k0sv1beta1.HelmExtensions
	}
	tests := []struct {
		name string
		args args
		want []k0sv1beta1.Chart
	}{
		{
			name: "no meta/configs",
			args: args{
				meta: nil,
				clusterconfig: k0sv1beta1.ClusterConfig{
					Spec: &k0sv1beta1.ClusterSpec{
						Extensions: &k0sv1beta1.ClusterExtensions{
							Helm: &k0sv1beta1.HelmExtensions{},
						},
					},
				},
				combinedConfigs: &k0sv1beta1.HelmExtensions{},
			},
			want: []k0sv1beta1.Chart{},
		},
		{
			name: "add new chart, change chart values, change chart version, remove old chart",
			args: args{
				meta: &release.Meta{
					Protected: map[string][]string{
						"changethis": {"abc"},
					},
				},
				clusterconfig: k0sv1beta1.ClusterConfig{
					Spec: &k0sv1beta1.ClusterSpec{
						Extensions: &k0sv1beta1.ClusterExtensions{
							Helm: &k0sv1beta1.HelmExtensions{
								Charts: []k0sv1beta1.Chart{
									{
										Name: "removethis",
									},
									{
										Name:   "changethis",
										Values: "abc: xyz",
									},
									{
										Name:    "newversion",
										Version: "1.0.0",
									},
								},
							},
						},
					},
				},
				combinedConfigs: &k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:   "addthis",
							Values: "addedval: xyz",
						},
						{
							Name:   "changethis",
							Values: "key2: val2",
						},
						{
							Name:    "newversion",
							Version: "2.0.0",
						},
					},
				},
			},
			want: []k0sv1beta1.Chart{
				{
					Name:   "addthis",
					Values: "addedval: xyz",
				},
				{
					Name:   "changethis",
					Values: "abc: xyz\nkey2: val2\n",
				},
				{
					Name:    "newversion",
					Version: "2.0.0",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			got, err := generateDesiredCharts(tt.args.meta, tt.args.clusterconfig, tt.args.combinedConfigs)
			req.NoError(err)
			req.ElementsMatch(tt.want, got)
		})
	}
}
