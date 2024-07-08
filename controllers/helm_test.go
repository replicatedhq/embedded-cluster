package controllers

import (
	"context"
	"testing"

	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster-kinds/types"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/registry"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_mergeHelmConfigs(t *testing.T) {
	type args struct {
		meta          *ectypes.ReleaseMetadata
		in            v1beta1.Extensions
		conditions    []metav1.Condition
		clusterConfig k0sv1beta1.ClusterConfig
	}
	tests := []struct {
		name             string
		args             args
		airgap           bool
		highAvailability bool
		disasterRecovery bool
		want             *k0sv1beta1.HelmExtensions
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
			name:             "disaster recovery enabled",
			disasterRecovery: true,
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
					BuiltinConfigs: map[string]k0sv1beta1.HelmExtensions{
						"seaweedfs": {
							Repositories: []k0sv1beta1.Repository{
								{
									Name: "seaweedfsrepo",
								},
							},
							Charts: []k0sv1beta1.Chart{
								{
									Name: "seaweedfschart",
									// Values: `{"filer":{"s3":{"existingConfigSecret":"seaweedfs-s3-secret"}}}`,
								},
							},
						},
						"registry": {
							Repositories: []k0sv1beta1.Repository{
								{
									Name: "registryrepo",
								},
							},
							Charts: []k0sv1beta1.Chart{
								{
									Name: "registrychart",
								},
							},
						},
						"registry-ha": {
							Repositories: []k0sv1beta1.Repository{
								{
									Name: "registryharepo",
								},
							},
							Charts: []k0sv1beta1.Chart{
								{
									Name: "registryhachart",
									// Values: `{"secrets":{"s3":{"secretRef":"registry-s3-secret"}}}`,
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
						Name: "registryrepo",
					},
				},
				Charts: []k0sv1beta1.Chart{
					{
						Name:  "origchart",
						Order: 100,
					},
					{
						Name:  "registrychart",
						Order: 100,
					},
				},
			},
		},
		{
			name:             "ha airgap enabled",
			airgap:           true,
			highAvailability: true,
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
						"seaweedfs": {
							Repositories: []k0sv1beta1.Repository{
								{
									Name: "seaweedfsrepo",
								},
							},
							Charts: []k0sv1beta1.Chart{
								{
									Name: "seaweedfschart",
									// Values: `{"filer":{"s3":{"existingConfigSecret":"seaweedfs-s3-secret"}}}`,
								},
							},
						},
						"registry": {
							Repositories: []k0sv1beta1.Repository{
								{
									Name: "registryrepo",
								},
							},
							Charts: []k0sv1beta1.Chart{
								{
									Name: "registrychart",
								},
							},
						},
						"registry-ha": {
							Repositories: []k0sv1beta1.Repository{
								{
									Name: "registryharepo",
								},
							},
							Charts: []k0sv1beta1.Chart{
								{
									Name: "registryhachart",
									// Values: `{"secrets":{"s3":{"secretRef":"registry-s3-secret"}}}`,
								},
							},
						},
					},
				},
				in: v1beta1.Extensions{},
				conditions: []metav1.Condition{
					{
						Type:   registry.RegistryMigrationStatusConditionType,
						Status: metav1.ConditionTrue,
						Reason: "MigrationJobCompleted",
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
						Name: "seaweedfsrepo",
					},
					{
						Name: "registryharepo",
					},
				},
				Charts: []k0sv1beta1.Chart{
					{
						Name:  "origchart",
						Order: 100,
					},
					{
						Name:  "seaweedfschart",
						Order: 100,
					},
					{
						Name:  "registryhachart",
						Order: 100,
					},
				},
			},
		},
		{
			name:             "ha airgap enabled, migration incomplete",
			airgap:           true,
			highAvailability: true,
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
						"seaweedfs": {
							Repositories: []k0sv1beta1.Repository{
								{
									Name: "seaweedfsrepo",
								},
							},
							Charts: []k0sv1beta1.Chart{
								{
									Name: "seaweedfschart",
									// Values: `{"filer":{"s3":{"existingConfigSecret":"seaweedfs-s3-secret"}}}`,
								},
							},
						},
						"registry": {
							Repositories: []k0sv1beta1.Repository{
								{
									Name: "registryrepo",
								},
							},
							Charts: []k0sv1beta1.Chart{
								{
									Name: "registrychart",
								},
							},
						},
						"registry-ha": {
							Repositories: []k0sv1beta1.Repository{
								{
									Name: "registryharepo",
								},
							},
							Charts: []k0sv1beta1.Chart{
								{
									Name: "registryhachart",
									// Values: `{"secrets":{"s3":{"secretRef":"registry-s3-secret"}}}`,
								},
							},
						},
					},
				},
				in: v1beta1.Extensions{},
				conditions: []metav1.Condition{
					{
						Type:   registry.RegistryMigrationStatusConditionType,
						Status: metav1.ConditionFalse,
						Reason: "MigrationInProgress",
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
						Name: "seaweedfsrepo",
					},
				},
				Charts: []k0sv1beta1.Chart{
					{
						Name:  "origchart",
						Order: 100,
					},
					{
						Name:  "seaweedfschart",
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
					AirGap:           tt.airgap,
					HighAvailability: tt.highAvailability,
					LicenseInfo: &v1beta1.LicenseInfo{
						IsDisasterRecoverySupported: tt.disasterRecovery,
					},
				},
				Status: v1beta1.InstallationStatus{
					Conditions: tt.args.conditions,
				},
			}

			req := require.New(t)
			got := mergeHelmConfigs(context.TODO(), tt.args.meta, &installation, tt.args.clusterConfig)
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

func Test_updateInfraChartsFromInstall(t *testing.T) {
	type args struct {
		in            *v1beta1.Installation
		clusterConfig k0sv1beta1.ClusterConfig
		charts        []k0sv1beta1.Chart
	}
	tests := []struct {
		name string
		args args
		want []k0sv1beta1.Chart
	}{
		{
			name: "other chart",
			args: args{
				in: &v1beta1.Installation{
					Spec: v1beta1.InstallationSpec{
						ClusterID: "abc",
					},
				},
				charts: []k0sv1beta1.Chart{
					{
						Name:   "test",
						Values: "abc: xyz",
					},
				},
			},
			want: []k0sv1beta1.Chart{
				{
					Name:   "test",
					Values: "abc: xyz",
				},
			},
		},
		{
			name: "admin console and operator",
			args: args{
				in: &v1beta1.Installation{
					Spec: v1beta1.InstallationSpec{
						ClusterID:        "testid",
						BinaryName:       "testbin",
						AirGap:           true,
						HighAvailability: true,
					},
				},
				charts: []k0sv1beta1.Chart{
					{
						Name:   "test",
						Values: "abc: xyz",
					},
					{
						Name:   "admin-console",
						Values: "abc: xyz",
					},
					{
						Name:   "embedded-cluster-operator",
						Values: "this: that",
					},
				},
			},
			want: []k0sv1beta1.Chart{
				{
					Name:   "test",
					Values: "abc: xyz",
				},
				{
					Name:   "admin-console",
					Values: "abc: xyz\nembeddedClusterID: testid\nisAirgap: \"true\"\nisHA: true\n",
				},
				{
					Name:   "embedded-cluster-operator",
					Values: "embeddedBinaryName: testbin\nembeddedClusterID: testid\nthis: that\n",
				},
			},
		},
		{
			name: "admin console and operator with proxy",
			args: args{
				in: &v1beta1.Installation{
					Spec: v1beta1.InstallationSpec{
						ClusterID:        "testid",
						BinaryName:       "testbin",
						AirGap:           false,
						HighAvailability: false,
						Proxy: &v1beta1.ProxySpec{
							HTTPProxy:  "http://proxy",
							HTTPSProxy: "https://proxy",
							NoProxy:    "noproxy",
						},
					},
				},
				charts: []k0sv1beta1.Chart{
					{
						Name:   "test",
						Values: "abc: xyz",
					},
					{
						Name:   "admin-console",
						Values: "abc: xyz",
					},
					{
						Name:   "embedded-cluster-operator",
						Values: "this: that",
					},
				},
			},
			want: []k0sv1beta1.Chart{
				{
					Name:   "test",
					Values: "abc: xyz",
				},
				{
					Name:   "admin-console",
					Values: "abc: xyz\nembeddedClusterID: testid\nextraEnv:\n- name: HTTP_PROXY\n  value: http://proxy\n- name: HTTPS_PROXY\n  value: https://proxy\n- name: NO_PROXY\n  value: noproxy\nisAirgap: \"false\"\nisHA: false\n",
				},
				{
					Name:   "embedded-cluster-operator",
					Values: "embeddedBinaryName: testbin\nembeddedClusterID: testid\nextraEnv:\n- name: HTTP_PROXY\n  value: http://proxy\n- name: HTTPS_PROXY\n  value: https://proxy\n- name: NO_PROXY\n  value: noproxy\nthis: that\n",
				},
			},
		},
		{
			name: "velero with proxy",
			args: args{
				in: &v1beta1.Installation{
					Spec: v1beta1.InstallationSpec{
						ClusterID:        "testid",
						BinaryName:       "testbin",
						AirGap:           false,
						HighAvailability: false,
						Proxy: &v1beta1.ProxySpec{
							HTTPProxy:  "http://proxy",
							HTTPSProxy: "https://proxy",
							NoProxy:    "noproxy",
						},
					},
				},
				charts: []k0sv1beta1.Chart{
					{
						Name:   "velero",
						Values: "abc: xyz\nconfiguration:\n  extraEnvVars: {}\n",
					},
				},
			},
			want: []k0sv1beta1.Chart{
				{
					Name:   "velero",
					Values: "abc: xyz\nconfiguration:\n  extraEnvVars:\n    HTTP_PROXY: http://proxy\n    HTTPS_PROXY: https://proxy\n    NO_PROXY: noproxy\n",
				},
			},
		},
		{
			name: "docker-registry",
			args: args{
				in: &v1beta1.Installation{
					Spec: v1beta1.InstallationSpec{
						ClusterID:  "testid",
						BinaryName: "testbin",
						AirGap:     true,
						Network:    &v1beta1.NetworkSpec{ServiceCIDR: "1.2.0.0/16"},
					},
				},
				clusterConfig: k0sv1beta1.ClusterConfig{},
				charts: []k0sv1beta1.Chart{
					{
						Name:   "docker-registry",
						Values: "this: that\nand: another\nservice:\n  clusterIP: \"abc\"\n",
					},
				},
			},
			want: []k0sv1beta1.Chart{
				{
					Name:   "docker-registry",
					Values: "and: another\nservice:\n  clusterIP: 1.2.0.11\nthis: that\n",
				},
			},
		},
		{
			name: "docker-registry ha",
			args: args{
				in: &v1beta1.Installation{
					Spec: v1beta1.InstallationSpec{
						ClusterID:        "testid",
						BinaryName:       "testbin",
						AirGap:           true,
						HighAvailability: true,
					},
				},
				clusterConfig: k0sv1beta1.ClusterConfig{},
				charts: []k0sv1beta1.Chart{
					{
						Name: "docker-registry",
						Values: `image:
  tag: 2.8.3
replicaCount: 2
s3:
  bucket: registry
  encrypt: false
  region: us-east-1
  regionEndpoint: DYNAMIC
  rootdirectory: /registry
  secure: false
secrets:
  s3:
    secretRef: seaweedfs-s3-rw`,
					},
				},
			},
			want: []k0sv1beta1.Chart{
				{
					Name: "docker-registry",
					Values: `image:
  tag: 2.8.3
replicaCount: 2
s3:
  bucket: registry
  encrypt: false
  region: us-east-1
  regionEndpoint: 10.96.0.12:8333
  rootdirectory: /registry
  secure: false
secrets:
  s3:
    secretRef: seaweedfs-s3-rw
`,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			got := updateInfraChartsFromInstall(context.TODO(), tt.args.in, tt.args.clusterConfig, tt.args.charts)
			req.ElementsMatch(tt.want, got)
		})
	}
}
