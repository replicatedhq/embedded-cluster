package controllers

import (
	"context"
	"testing"

	k0shelmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster-kinds/types"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/release"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInstallationReconciler_ReconcileHelmCharts(t *testing.T) {
	type fields struct {
		State     []runtime.Object
		Discovery discovery.DiscoveryInterface
		Scheme    *runtime.Scheme
	}
	tests := []struct {
		name        string
		fields      fields
		in          v1beta1.Installation
		out         v1beta1.InstallationStatus
		releaseMeta ectypes.ReleaseMetadata
		updatedHelm *k0sv1beta1.HelmExtensions
	}{
		{
			name: "no input config, move to installed",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
			},
			out: v1beta1.InstallationStatus{State: v1beta1.InstallationStateInstalled, Reason: "Installed"},
		},
		{
			name: "k8s install in progress, no state change",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateInstalling},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "abc",
					},
				},
			},
			out: v1beta1.InstallationStatus{State: v1beta1.InstallationStateInstalling},
		},
		{
			name: "k8s install completed, good version, no charts",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateInstalled,
				Reason: "Installed",
			},
			releaseMeta: ectypes.ReleaseMetadata{K0sSHA: "abc"},
		},
		{
			name: "k8s install completed, good version, only config extensions chart",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &k0sv1beta1.HelmExtensions{
								Charts: []k0sv1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateInstalled,
				Reason: "Installed",
			},
			releaseMeta: ectypes.ReleaseMetadata{K0sSHA: "abc"},
		},
		{
			name: "k8s install completed, good version, both types of charts, no drift",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &k0sv1beta1.HelmExtensions{
								Charts: []k0sv1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateInstalled,
				Reason: "Addons upgraded",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "metachart"},
						Status: k0shelmv1beta1.ChartStatus{Version: "1", ValuesHash: "ad5c2ed66264c2cd2cf47c408159c035bde16fec4e15a7e7659a6ad0cd87935c"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "extchart",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "extchart"},
						Status: k0shelmv1beta1.ChartStatus{Version: "2", ValuesHash: "c687e5ae3f4a71927fb7ba3a3fee85f40c2debeec3b8bf66d038955a60ccf3ba"},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
										},
										{
											Name:    "extchart",
											Version: "2",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, good version, both types of charts, chart errors",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &k0sv1beta1.HelmExtensions{
								Charts: []k0sv1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateHelmChartUpdateFailure,
				Reason: "failed to update helm charts: exterror",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{ReleaseName: "metachart"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "extchart",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "extchart"},
						Status: k0shelmv1beta1.ChartStatus{Version: "2", Error: "exterror"},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
										},
										{
											Name:    "extchart",
											Version: "2",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, good version, releaseMeta chart, chart errors",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateHelmChartUpdateFailure,
				Reason: "failed to update helm charts: metaerror",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "metachart"},
						Status: k0shelmv1beta1.ChartStatus{Version: "1", Error: "metaerror"},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, good version, both types of charts, drift, addons already installing",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateAddonsInstalling},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &k0sv1beta1.HelmExtensions{
								Charts: []k0sv1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State: v1beta1.InstallationStateAddonsInstalling,
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, good version, both types of charts, drift",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &k0sv1beta1.HelmExtensions{
								Charts: []k0sv1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateAddonsInstalling,
				Reason: "Installing addons",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Order:   1,
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{},
							},
						},
					},
				},
			},
			updatedHelm: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "metachart",
						Version: "1",
						Order:   101,
					},
					{
						Name:    "extchart",
						Version: "2",
						Order:   110,
					},
				},
			},
		},
		{
			name: "k8s install completed, good version, overridden values, both types of charts, no drift",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &k0sv1beta1.HelmExtensions{
								Charts: []k0sv1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateInstalled,
				Reason: "Addons upgraded",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Values: `
abc: xyz
password: overridden`,
						},
					},
				},
				Protected: map[string][]string{
					"metachart": {"password"},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "metachart",
							Values: `abc: xyz
password: original`,
						},
						Status: k0shelmv1beta1.ChartStatus{Version: "1", ValuesHash: "eec6dc8e36073ed2211154bca2d54cdc01acba8f512d46c095c3d7a1ede4b0d6"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "extchart",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "extchart"},
						Status: k0shelmv1beta1.ChartStatus{Version: "2", ValuesHash: "c687e5ae3f4a71927fb7ba3a3fee85f40c2debeec3b8bf66d038955a60ccf3ba"},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
											Values: `
abc: xyz
password: original`,
										},
										{
											Name:    "extchart",
											Version: "2",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, good version, overridden values, both types of charts, values drift",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &k0sv1beta1.HelmExtensions{
								Charts: []k0sv1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateAddonsInstalling,
				Reason: "Installing addons",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Values: `
abc: xyz
password: overridden`,
						},
					},
				},
				Protected: map[string][]string{
					"metachart": {"password"},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "metachart",
							Values: `abc: original
password: original`,
						},
						Status: k0shelmv1beta1.ChartStatus{
							Version:     "1",
							ReleaseName: "metachart",
							ValuesHash:  "1fcf324bc7890a68f7402a7a523bb47a470b726f1011f69c3d7cf2e911f15685",
						},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "extchart",
						},
						Spec: k0shelmv1beta1.ChartSpec{ReleaseName: "extchart"},
						Status: k0shelmv1beta1.ChartStatus{
							Version:     "2",
							ReleaseName: "extchart",
							ValuesHash:  "c687e5ae3f4a71927fb7ba3a3fee85f40c2debeec3b8bf66d038955a60ccf3ba",
						},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
											Values: `
abc: original
password: original`,
										},
										{
											Name:    "extchart",
											Version: "2",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, values drift but chart not yet installed",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStatePendingChartCreation,
				Reason: "Pending charts: [metachart]",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Values:  `abc: xyz`,
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "metachart",
							Values:      `abc: original`,
						},
						Status: k0shelmv1beta1.ChartStatus{},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
											Values:  `abc: original`,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, no values drift but chart not yet installed",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStatePendingChartCreation,
				Reason: "Pending charts: [metachart]",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Values:  `abc: xyz`,
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "metachart",
							Version:     "1",
							Values:      `abc: original`,
						},
						Status: k0shelmv1beta1.ChartStatus{
							ReleaseName: "metachart",
						},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
											Values:  `abc: xyz`,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, updating charts despite errors",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateAddonsInstalling,
				Reason: "Installing addons",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Values:  `abc: xyz`,
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "metachart",
							Version:     "1",
							Values:      `abc: original`,
						},
						Status: k0shelmv1beta1.ChartStatus{
							ReleaseName: "metachart",
							Error:       "error",
						},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
											Values:  `abc: original`,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, no values drift",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateInstalled,
				Reason: "Addons upgraded",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: k0sv1beta1.HelmExtensions{
					Charts: []k0sv1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Values:  `abc: xyz`,
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "metachart",
							Version:     "1",
							Values:      `abc: xyz`,
						},
						Status: k0shelmv1beta1.ChartStatus{
							ReleaseName: "metachart",
							Version:     "1",
							ValuesHash:  "dace29a7a92865fa8a5dcd85540a806aa9cf0a7d37fa119f2546a17afd7e33b4",
						},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
											Values:  `abc: xyz`,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			release.CacheMeta("goodver", tt.releaseMeta)

			sch, err := k0shelmv1beta1.SchemeBuilder.Build()
			req.NoError(err)
			req.NoError(k0sv1beta1.AddToScheme(sch))
			fakeCli := fake.NewClientBuilder().WithScheme(sch).WithRuntimeObjects(tt.fields.State...).Build()

			r := &InstallationReconciler{
				Client:    fakeCli,
				Discovery: tt.fields.Discovery,
				Scheme:    tt.fields.Scheme,
			}
			err = r.ReconcileHelmCharts(context.Background(), &tt.in)
			req.NoError(err)
			req.Equal(tt.out, tt.in.Status)

			if tt.updatedHelm != nil {
				var gotCluster k0sv1beta1.ClusterConfig
				err = fakeCli.Get(context.Background(), client.ObjectKey{Name: "k0s", Namespace: "kube-system"}, &gotCluster)
				req.NoError(err)
				req.ElementsMatch(tt.updatedHelm.Charts, gotCluster.Spec.Extensions.Helm.Charts)
				req.ElementsMatch(tt.updatedHelm.Repositories, gotCluster.Spec.Extensions.Helm.Repositories)
			}
		})
	}
}
