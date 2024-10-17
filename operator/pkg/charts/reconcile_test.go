package charts

import (
	"context"
	"k8s.io/utils/ptr"
	"testing"

	k0shelmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInstallationReconciler_ReconcileHelmCharts(t *testing.T) {
	test_replaceAddonMeta()
	defer test_restoreAddonMeta()

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
			name: "k8s install completed, good version, both types of charts, no drift",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &v1beta1.Helm{
								Charts: []v1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
					ClusterID:  "e79f0701-67f3-4abf-a672-42a1f3ed231b",
					BinaryName: "test-binary-name",
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateInstalled,
				Reason: "Addons upgraded",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Images: []string{
					"abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0",
					"docker.io/replicated/another-image:latest-arm64@sha256:a9ab9db181f9898283a87be0f79d85cb8f3d22a790b71f52c8a9d339e225dedd",
					"docker.io/replicated/embedded-cluster-operator-image:latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8",
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "extchart",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "extchart"},
						Status: k0shelmv1beta1.ChartStatus{Version: "2", ValuesHash: "c687e5ae3f4a71927fb7ba3a3fee85f40c2debeec3b8bf66d038955a60ccf3ba"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "openebs",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "openebs", Values: test_openebsValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-openebs", ValuesHash: "c0ea0af176f78196117571c1a50f6f679da08a2975e442fe39542cbe419b55c6"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "embedded-cluster-operator",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "embedded-cluster-operator", Values: test_operatorValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-operator", ValuesHash: "215c33c6a56953b6d6814251f6fa0e78d3884a4d15dbb515a3942baf40900893"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin-console",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "admin-console", Values: test_onlineAdminConsoleValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-admin-console", ValuesHash: "88e04728e85bbbf8a7c676a28c6bc7809273c8a0aa21ed0a407c635855b6944e"},
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
											Name:    "extchart",
											Version: "2",
										},
										{
											Name:         "openebs",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
											Version:      "1.2.3-openebs",
											Values:       test_openebsValues,
											TargetNS:     "openebs",
											ForceUpgrade: ptr.To(false),
											Order:        101,
										},
										{
											Name:         "embedded-cluster-operator",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
											Version:      "1.2.3-operator",
											Values:       test_operatorValues,
											TargetNS:     "embedded-cluster",
											ForceUpgrade: ptr.To(false),
											Order:        103,
										},
										{
											Name:         "admin-console",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
											Version:      "1.2.3-admin-console",
											Values:       test_onlineAdminConsoleValues,
											TargetNS:     "kotsadm",
											ForceUpgrade: ptr.To(false),
											Order:        105,
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
							Helm: &v1beta1.Helm{
								Charts: []v1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
					ClusterID:  "e79f0701-67f3-4abf-a672-42a1f3ed231b",
					BinaryName: "test-binary-name",
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateHelmChartUpdateFailure,
				Reason: "failed to update helm charts: \nextchart: exterror\n",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Images: []string{
					"abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0",
					"docker.io/replicated/another-image:latest-arm64@sha256:a9ab9db181f9898283a87be0f79d85cb8f3d22a790b71f52c8a9d339e225dedd",
					"docker.io/replicated/embedded-cluster-operator-image:latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8",
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "extchart",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "extchart"},
						Status: k0shelmv1beta1.ChartStatus{Version: "2", Error: "exterror"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "openebs",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "openebs", Values: test_openebsValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-openebs", ValuesHash: "c0ea0af176f78196117571c1a50f6f679da08a2975e442fe39542cbe419b55c6"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "embedded-cluster-operator",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "embedded-cluster-operator", Values: test_operatorValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-operator", ValuesHash: "215c33c6a56953b6d6814251f6fa0e78d3884a4d15dbb515a3942baf40900893"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin-console",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "admin-console", Values: test_onlineAdminConsoleValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-admin-console", ValuesHash: "88e04728e85bbbf8a7c676a28c6bc7809273c8a0aa21ed0a407c635855b6944e"},
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
											Name:    "extchart",
											Version: "2",
										},
										{
											Name:         "openebs",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
											Version:      "1.2.3-openebs",
											Values:       test_openebsValues,
											TargetNS:     "openebs",
											ForceUpgrade: ptr.To(false),
											Order:        101,
										},
										{
											Name:         "embedded-cluster-operator",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
											Version:      "1.2.3-operator",
											Values:       test_operatorValues,
											TargetNS:     "embedded-cluster",
											ForceUpgrade: ptr.To(false),
											Order:        103,
										},
										{
											Name:         "admin-console",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
											Version:      "1.2.3-admin-console",
											Values:       test_onlineAdminConsoleValues,
											TargetNS:     "kotsadm",
											ForceUpgrade: ptr.To(false),
											Order:        105,
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
			name: "k8s install completed, good version, builtin chart, chart errors",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
					},
					ClusterID:  "e79f0701-67f3-4abf-a672-42a1f3ed231b",
					BinaryName: "test-binary-name",
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateHelmChartUpdateFailure,
				Reason: "failed to update helm charts: \nopenebs: openerror\n",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Images: []string{
					"abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0",
					"docker.io/replicated/another-image:latest-arm64@sha256:a9ab9db181f9898283a87be0f79d85cb8f3d22a790b71f52c8a9d339e225dedd",
					"docker.io/replicated/embedded-cluster-operator-image:latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8",
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
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "openebs",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "openebs", Values: test_openebsValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-openebs", Error: "openerror"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "embedded-cluster-operator",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "embedded-cluster-operator", Values: test_operatorValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-operator", ValuesHash: "215c33c6a56953b6d6814251f6fa0e78d3884a4d15dbb515a3942baf40900893"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin-console",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "admin-console", Values: test_onlineAdminConsoleValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-admin-console", ValuesHash: "88e04728e85bbbf8a7c676a28c6bc7809273c8a0aa21ed0a407c635855b6944e"},
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
											Name:         "openebs",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
											Version:      "1.2.3-openebs",
											Values:       test_openebsValues,
											TargetNS:     "openebs",
											ForceUpgrade: ptr.To(false),
											Order:        101,
										},
										{
											Name:         "embedded-cluster-operator",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
											Version:      "1.2.3-operator",
											Values:       test_operatorValues,
											TargetNS:     "embedded-cluster",
											ForceUpgrade: ptr.To(false),
											Order:        103,
										},
										{
											Name:         "admin-console",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
											Version:      "1.2.3-admin-console",
											Values:       test_onlineAdminConsoleValues,
											TargetNS:     "kotsadm",
											ForceUpgrade: ptr.To(false),
											Order:        105,
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
							Helm: &v1beta1.Helm{
								Charts: []v1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
					ClusterID:  "e79f0701-67f3-4abf-a672-42a1f3ed231b",
					BinaryName: "test-binary-name",
				},
			},
			out: v1beta1.InstallationStatus{
				State: v1beta1.InstallationStateAddonsInstalling,
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Images: []string{
					"abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0",
					"docker.io/replicated/another-image:latest-arm64@sha256:a9ab9db181f9898283a87be0f79d85cb8f3d22a790b71f52c8a9d339e225dedd",
					"docker.io/replicated/embedded-cluster-operator-image:latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8",
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
							Helm: &v1beta1.Helm{
								Charts: []v1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
					ClusterID:  "e79f0701-67f3-4abf-a672-42a1f3ed231b",
					BinaryName: "test-binary-name",
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateAddonsInstalling,
				Reason: "Installing addons",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Images: []string{
					"abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0",
					"docker.io/replicated/another-image:latest-arm64@sha256:a9ab9db181f9898283a87be0f79d85cb8f3d22a790b71f52c8a9d339e225dedd",
					"docker.io/replicated/embedded-cluster-operator-image:latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8",
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
						Name:    "extchart",
						Version: "2",
						Order:   110,
					},
					{
						Name:         "openebs",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
						Version:      "1.2.3-openebs",
						Values:       test_openebsValues,
						TargetNS:     "openebs",
						ForceUpgrade: ptr.To(false),
						Order:        101,
					},
					{
						Name:         "embedded-cluster-operator",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
						Version:      "1.2.3-operator",
						Values:       test_operatorValues,
						TargetNS:     "embedded-cluster",
						ForceUpgrade: ptr.To(false),
						Order:        103,
					},
					{
						Name:         "admin-console",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
						Version:      "1.2.3-admin-console",
						Values:       test_onlineAdminConsoleValues,
						TargetNS:     "kotsadm",
						ForceUpgrade: ptr.To(false),
						Order:        105,
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
							Helm: &v1beta1.Helm{
								Charts: []v1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
						UnsupportedOverrides: v1beta1.UnsupportedOverrides{
							BuiltInExtensions: []v1beta1.BuiltInExtension{
								{
									Name:   "admin-console",
									Values: `embeddedClusterVersion: abctest`,
								},
								{
									Name:   "embedded-cluster-operator",
									Values: `embeddedClusterVersion: abctest`,
								},
							},
						},
					},
					ClusterID:  "e79f0701-67f3-4abf-a672-42a1f3ed231b",
					BinaryName: "test-binary-name",
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateAddonsInstalling,
				Reason: "Installing addons",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Images: []string{
					"abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0",
					"docker.io/replicated/another-image:latest-arm64@sha256:a9ab9db181f9898283a87be0f79d85cb8f3d22a790b71f52c8a9d339e225dedd",
					"docker.io/replicated/embedded-cluster-operator-image:latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8",
				},
			},
			fields: fields{
				State: []runtime.Object{
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
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "openebs",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "openebs", Values: test_openebsValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-openebs", ValuesHash: "c0ea0af176f78196117571c1a50f6f679da08a2975e442fe39542cbe419b55c6"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "embedded-cluster-operator",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "embedded-cluster-operator", Values: test_operatorValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-operator", ValuesHash: "215c33c6a56953b6d6814251f6fa0e78d3884a4d15dbb515a3942baf40900893"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin-console",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "admin-console", Values: test_onlineAdminConsoleValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-admin-console", ValuesHash: "88e04728e85bbbf8a7c676a28c6bc7809273c8a0aa21ed0a407c635855b6944e"},
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
											Name:    "extchart",
											Version: "2",
										},
										{
											Name:         "openebs",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
											Version:      "1.2.3-openebs",
											Values:       test_openebsValues,
											TargetNS:     "openebs",
											ForceUpgrade: ptr.To(false),
											Order:        101,
										},
										{
											Name:         "embedded-cluster-operator",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
											Version:      "1.2.3-operator",
											Values:       test_operatorValues,
											TargetNS:     "embedded-cluster",
											ForceUpgrade: ptr.To(false),
											Order:        103,
										},
										{
											Name:         "admin-console",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
											Version:      "1.2.3-admin-console",
											Values:       test_onlineAdminConsoleValues,
											TargetNS:     "kotsadm",
											ForceUpgrade: ptr.To(false),
											Order:        105,
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
			name: "k8s install completed, good version, overridden values, both types of charts, no values drift",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateAddonsInstalling},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &v1beta1.Helm{
								Charts: []v1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
						UnsupportedOverrides: v1beta1.UnsupportedOverrides{
							BuiltInExtensions: []v1beta1.BuiltInExtension{
								{
									Name:   "admin-console",
									Values: `embeddedClusterVersion: abctest`,
								},
								{
									Name:   "embedded-cluster-operator",
									Values: `embeddedClusterVersion: abctest`,
								},
							},
						},
					},
					ClusterID:  "e79f0701-67f3-4abf-a672-42a1f3ed231b",
					BinaryName: "test-binary-name",
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateInstalled,
				Reason: "Addons upgraded",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Images: []string{
					"abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0",
					"docker.io/replicated/another-image:latest-arm64@sha256:a9ab9db181f9898283a87be0f79d85cb8f3d22a790b71f52c8a9d339e225dedd",
					"docker.io/replicated/embedded-cluster-operator-image:latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8",
				},
			},
			fields: fields{
				State: []runtime.Object{
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
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "openebs",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "openebs", Values: test_openebsValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-openebs", ValuesHash: "c0ea0af176f78196117571c1a50f6f679da08a2975e442fe39542cbe419b55c6"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "embedded-cluster-operator",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "embedded-cluster-operator", Values: test_overriddenOperatorValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-operator", ValuesHash: "b2ece3c59578e31d8a8d997923de5971dd04c3d417366af115464c79070ad3b3"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin-console",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "admin-console", Values: test_overriddenOnlineAdminConsoleValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-admin-console", ValuesHash: "1a02a40c607e2addad17e289e6d6d155ab4e7b0b7dd7e153fb123032812c5227"},
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
											Name:    "extchart",
											Version: "2",
										},
										{
											Name:         "openebs",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
											Version:      "1.2.3-openebs",
											Values:       test_openebsValues,
											TargetNS:     "openebs",
											ForceUpgrade: ptr.To(false),
											Order:        101,
										},
										{
											Name:         "embedded-cluster-operator",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
											Version:      "1.2.3-operator",
											Values:       test_overriddenOperatorValues,
											TargetNS:     "embedded-cluster",
											ForceUpgrade: ptr.To(false),
											Order:        103,
										},
										{
											Name:         "admin-console",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
											Version:      "1.2.3-admin-console",
											Values:       test_overriddenOnlineAdminConsoleValues,
											TargetNS:     "kotsadm",
											ForceUpgrade: ptr.To(false),
											Order:        105,
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
					ClusterID:  "e79f0701-67f3-4abf-a672-42a1f3ed231b",
					BinaryName: "test-binary-name",
				},
			},
			out: v1beta1.InstallationStatus{
				State:         v1beta1.InstallationStatePendingChartCreation,
				Reason:        "Pending charts: [openebs embedded-cluster-operator admin-console]",
				PendingCharts: []string{"openebs", "embedded-cluster-operator", "admin-console"},
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Images: []string{
					"abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0",
					"docker.io/replicated/another-image:latest-arm64@sha256:a9ab9db181f9898283a87be0f79d85cb8f3d22a790b71f52c8a9d339e225dedd",
					"docker.io/replicated/embedded-cluster-operator-image:latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8",
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "openebs",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "openebs"},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-openebs"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "embedded-cluster-operator",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "embedded-cluster-operator"},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-operator"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin-console",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "admin-console"},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-admin-console"},
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
											Name:         "openebs",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
											Version:      "1.2.3-openebs",
											TargetNS:     "openebs",
											ForceUpgrade: ptr.To(false),
											Order:        101,
										},
										{
											Name:         "embedded-cluster-operator",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
											Version:      "1.2.3-operator",
											TargetNS:     "embedded-cluster",
											ForceUpgrade: ptr.To(false),
											Order:        103,
										},
										{
											Name:         "admin-console",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
											Version:      "1.2.3-admin-console",
											TargetNS:     "kotsadm",
											ForceUpgrade: ptr.To(false),
											Order:        105,
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
					ClusterID:  "e79f0701-67f3-4abf-a672-42a1f3ed231b",
					BinaryName: "test-binary-name",
				},
			},
			out: v1beta1.InstallationStatus{
				State:         v1beta1.InstallationStatePendingChartCreation,
				Reason:        "Pending charts: [openebs embedded-cluster-operator admin-console]",
				PendingCharts: []string{"openebs", "embedded-cluster-operator", "admin-console"},
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Images: []string{
					"abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0",
					"docker.io/replicated/another-image:latest-arm64@sha256:a9ab9db181f9898283a87be0f79d85cb8f3d22a790b71f52c8a9d339e225dedd",
					"docker.io/replicated/embedded-cluster-operator-image:latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8",
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "openebs",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "openebs", Values: test_openebsValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-openebs"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "embedded-cluster-operator",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "embedded-cluster-operator", Values: test_operatorValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-operator"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin-console",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "admin-console", Values: test_onlineAdminConsoleValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-admin-console"},
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
											Name:         "openebs",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
											Version:      "1.2.3-openebs",
											Values:       test_openebsValues,
											TargetNS:     "openebs",
											ForceUpgrade: ptr.To(false),
											Order:        101,
										},
										{
											Name:         "embedded-cluster-operator",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
											Version:      "1.2.3-operator",
											Values:       test_operatorValues,
											TargetNS:     "embedded-cluster",
											ForceUpgrade: ptr.To(false),
											Order:        103,
										},
										{
											Name:         "admin-console",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
											Version:      "1.2.3-admin-console",
											Values:       test_onlineAdminConsoleValues,
											TargetNS:     "kotsadm",
											ForceUpgrade: ptr.To(false),
											Order:        105,
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
					ClusterID:  "e79f0701-67f3-4abf-a672-42a1f3ed231b",
					BinaryName: "test-binary-name",
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateAddonsInstalling,
				Reason: "Installing addons",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Images: []string{
					"abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0",
					"docker.io/replicated/another-image:latest-arm64@sha256:a9ab9db181f9898283a87be0f79d85cb8f3d22a790b71f52c8a9d339e225dedd",
					"docker.io/replicated/embedded-cluster-operator-image:latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8",
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "openebs",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "openebs", Values: "oldkeys: oldvalues"},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-openebs", Error: "openerror"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "embedded-cluster-operator",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "embedded-cluster-operator", Values: test_operatorValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-operator", ValuesHash: "215c33c6a56953b6d6814251f6fa0e78d3884a4d15dbb515a3942baf40900893"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin-console",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "admin-console", Values: test_onlineAdminConsoleValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-admin-console", ValuesHash: "88e04728e85bbbf8a7c676a28c6bc7809273c8a0aa21ed0a407c635855b6944e"},
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
											Name:         "openebs",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
											Version:      "1.2.3-openebs",
											Values:       "oldkeys: oldvalues",
											TargetNS:     "openebs",
											ForceUpgrade: ptr.To(false),
											Order:        101,
										},
										{
											Name:         "embedded-cluster-operator",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
											Version:      "1.2.3-operator",
											Values:       test_operatorValues,
											TargetNS:     "embedded-cluster",
											ForceUpgrade: ptr.To(false),
											Order:        103,
										},
										{
											Name:         "admin-console",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
											Version:      "1.2.3-admin-console",
											Values:       test_onlineAdminConsoleValues,
											TargetNS:     "kotsadm",
											ForceUpgrade: ptr.To(false),
											Order:        105,
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

			sch := runtime.NewScheme()
			req.NoError(k0sv1beta1.AddToScheme(sch))
			req.NoError(k0shelmv1beta1.AddToScheme(sch))
			fakeCli := fake.NewClientBuilder().WithScheme(sch).WithRuntimeObjects(tt.fields.State...).Build()

			_, err := ReconcileHelmCharts(context.Background(), fakeCli, &tt.in)
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
