package controllers

import (
	"testing"

	"github.com/k0sproject/dig"
	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster-kinds/types"
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
		meta *ectypes.ReleaseMetadata
		in   v1beta1.Extensions
	}
	tests := []struct {
		name      string
		args      args
		airgap    bool
		snapshots bool
		want      *k0sv1beta1.HelmExtensions
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
								Order:   2,
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
						Order:   102,
					},
				},
			},
		},
		{
			name: "add new chart + repo",
			args: args{
				meta: &ectypes.ReleaseMetadata{
					Configs: k0sv1beta1.HelmExtensions{
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
					AirgapConfigs: k0sv1beta1.HelmExtensions{
						Charts: []k0sv1beta1.Chart{
							{
								Name: "airgapchart",
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
						Name:  "origchart",
						Order: 110,
					},
					{
						Name:    "newchart",
						Version: "1.0.0",
						Order:   110,
					},
				},
			},
		},
		{
			name:   "airgap enabled",
			airgap: true,
			args: args{
				meta: &ectypes.ReleaseMetadata{
					Configs: k0sv1beta1.HelmExtensions{
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
					AirgapConfigs: k0sv1beta1.HelmExtensions{
						Charts: []k0sv1beta1.Chart{
							{
								Name: "airgapchart",
							},
						},
					},
				},
				in: v1beta1.Extensions{},
			},
			want: &k0sv1beta1.HelmExtensions{
				ConcurrencyLevel: 1,
				Repositories: []k0sv1beta1.Repository{
					{
						Name: "origrepo",
					},
				},
				Charts: []k0sv1beta1.Chart{
					{
						Name:  "origchart",
						Order: 100,
					},
					{
						Name:  "airgapchart",
						Order: 100,
					},
				},
			},
		},
		{
			name:      "snapshots enabled",
			snapshots: true,
			args: args{
				meta: &ectypes.ReleaseMetadata{
					Configs: k0sv1beta1.HelmExtensions{
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
					BuiltinConfigs: map[string]k0sv1beta1.HelmExtensions{
						"velero": {
							Repositories: []k0sv1beta1.Repository{
								{
									Name: "velerorepo",
								},
							},
							Charts: []k0sv1beta1.Chart{
								{
									Name: "velerochart",
								},
							},
						},
					},
				},
				in: v1beta1.Extensions{},
			},
			want: &k0sv1beta1.HelmExtensions{
				ConcurrencyLevel: 1,
				Repositories: []k0sv1beta1.Repository{
					{
						Name: "origrepo",
					},
					{
						Name: "velerorepo",
					},
				},
				Charts: []k0sv1beta1.Chart{
					{
						Name:  "origchart",
						Order: 100,
					},
					{
						Name:  "velerochart",
						Order: 100,
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
					AirGap: tt.airgap,
					LicenseInfo: &v1beta1.LicenseInfo{
						IsSnapshotSupported: tt.snapshots,
					},
				},
			}

			req := require.New(t)
			got := mergeHelmConfigs(tt.args.meta, &installation)
			req.Equal(tt.want, got)
		})
	}
}

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

func Test_generateDesiredCharts(t *testing.T) {
	type args struct {
		meta            *ectypes.ReleaseMetadata
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
			name: "add new chart, change chart values, change chart versions, remove old chart",
			args: args{
				meta: &ectypes.ReleaseMetadata{
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
									{
										Name:    "change2",
										Values:  "",
										Version: "1.0.0",
									},
									{
										Name:    "change3",
										Values:  "",
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
						{
							Name:    "change2",
							Values:  "abc: 2",
							Version: "1.0.2",
						},
						{
							Name:    "change3",
							Version: "1.0.3",
							Values:  "abc: 3",
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
				{
					Name:    "change2",
					Values:  "abc: 2",
					Version: "1.0.2",
				},
				{
					Name:    "change3",
					Version: "1.0.3",
					Values:  "abc: 3",
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

func Test_applyUserProvidedAddonOverrides(t *testing.T) {
	tests := []struct {
		name         string
		installation *v1beta1.Installation
		config       *k0sv1beta1.HelmExtensions
		want         *k0sv1beta1.HelmExtensions
	}{
		{
			name:         "no config",
			installation: &v1beta1.Installation{},
			config: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
			want: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
		},
		{
			name: "no override",
			installation: &v1beta1.Installation{
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						UnsupportedOverrides: v1beta1.UnsupportedOverrides{
							BuiltInExtensions: []v1beta1.BuiltInExtension{},
						},
					},
				},
			},
			config: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
			want: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
		},
		{
			name: "single addition",
			installation: &v1beta1.Installation{
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						UnsupportedOverrides: v1beta1.UnsupportedOverrides{
							BuiltInExtensions: []v1beta1.BuiltInExtension{
								{
									Name:   "test",
									Values: "foo: bar",
								},
							},
						},
					},
				},
			},
			config: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
			want: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz\nfoo: bar\n",
					},
				},
			},
		},
		{
			name: "single override",
			installation: &v1beta1.Installation{
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						UnsupportedOverrides: v1beta1.UnsupportedOverrides{
							BuiltInExtensions: []v1beta1.BuiltInExtension{
								{
									Name:   "test",
									Values: "abc: newvalue",
								},
							},
						},
					},
				},
			},
			config: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
			want: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: newvalue\n",
					},
				},
			},
		},
		{
			name: "multiple additions and overrides",
			installation: &v1beta1.Installation{
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						UnsupportedOverrides: v1beta1.UnsupportedOverrides{
							BuiltInExtensions: []v1beta1.BuiltInExtension{
								{
									Name:   "chart0",
									Values: "added: added\noverridden: overridden",
								},
								{
									Name:   "chart1",
									Values: "foo: replacement",
								},
							},
						},
					},
				},
			},
			config: &k0sv1beta1.HelmExtensions{
				ConcurrencyLevel: 999,
				Repositories: []k0sv1beta1.Repository{
					{
						Name: "repo",
						URL:  "https://repo",
					},
				},
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "chart0",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
					{
						Name:    "chart1",
						Version: "1.0.0",
						Values:  "foo: bar",
					},
				},
			},
			want: &k0sv1beta1.HelmExtensions{
				ConcurrencyLevel: 999,
				Repositories: []k0sv1beta1.Repository{
					{
						Name: "repo",
						URL:  "https://repo",
					},
				},
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "chart0",
						Version: "1.0.0",
						Values:  "abc: xyz\nadded: added\noverridden: overridden\n",
					},
					{
						Name:    "chart1",
						Version: "1.0.0",
						Values:  "foo: replacement\n",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			got, err := applyUserProvidedAddonOverrides(tt.installation, tt.config)
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}
