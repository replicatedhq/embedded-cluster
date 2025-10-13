package extensions

import (
	"context"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpgrade(t *testing.T) {
	// Discard log messages
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))

	tests := []struct {
		name             string
		prev             *ecv1beta1.Installation
		in               *ecv1beta1.Installation
		setupMockHelmCli func(t *testing.T) *helm.MockClient
		validateIn       func(t *testing.T, in *ecv1beta1.Installation)
		wantErr          bool
	}{
		{
			name: "install if release does not exist",
			prev: nil,
			in: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{
							Helm: &ecv1beta1.Helm{
								Charts: []ecv1beta1.Chart{
									{
										Name:      "existing-chart",
										ChartName: "test/chart",
										Version:   "1.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns",
										Order:     1,
									},
								},
							},
						},
					},
				},
			},
			setupMockHelmCli: func(t *testing.T) *helm.MockClient {
				helmCli := &helm.MockClient{}
				mock.InOrder(
					helmCli.
						On("ReleaseExists", mock.Anything, "test-ns", "existing-chart").
						Once().
						Return(false, nil),
					helmCli.
						On("Install", mock.Anything, helm.InstallOptions{
							ReleaseName:  "existing-chart",
							ChartPath:    "test/chart",
							ChartVersion: "1.0.0",
							Values:       map[string]interface{}{"abc": "xyz"},
							Namespace:    "test-ns",
						}).
						Once().
						Return(nil, nil),
				)
				return helmCli
			},
			wantErr: false,
			validateIn: func(t *testing.T, in *ecv1beta1.Installation) {
				assert.Len(t, in.Status.Conditions, 1, "expected 1 condition")
				assert.Equal(t, "test-ns-existing-chart", in.Status.Conditions[0].Type, "expected condition type")
				assert.Equal(t, metav1.ConditionTrue, in.Status.Conditions[0].Status, "expected condition status")
				assert.Equal(t, "Installed", in.Status.Conditions[0].Reason, "expected condition reason")
			},
		},
		{
			name: "skip install if release exists",
			prev: nil,
			in: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{
							Helm: &ecv1beta1.Helm{
								Charts: []ecv1beta1.Chart{
									{
										Name:      "existing-chart",
										ChartName: "test/chart",
										Version:   "1.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns",
										Order:     1,
									},
								},
							},
						},
					},
				},
			},
			setupMockHelmCli: func(t *testing.T) *helm.MockClient {
				helmCli := &helm.MockClient{}
				mock.InOrder(
					helmCli.
						On("ReleaseExists", mock.Anything, "test-ns", "existing-chart").
						Once().
						Return(true, nil),
				)
				return helmCli
			},
			validateIn: func(t *testing.T, in *ecv1beta1.Installation) {
				assert.Len(t, in.Status.Conditions, 1, "expected 1 condition")
				assert.Equal(t, "test-ns-existing-chart", in.Status.Conditions[0].Type, "expected condition type")
				assert.Equal(t, metav1.ConditionTrue, in.Status.Conditions[0].Status, "expected condition status")
				assert.Equal(t, "Installed", in.Status.Conditions[0].Reason, "expected condition reason")
			},
			wantErr: false,
		},
		{
			name: "upgrade if changes",
			prev: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{
							Helm: &ecv1beta1.Helm{
								Charts: []ecv1beta1.Chart{
									{
										Name:      "test-chart",
										ChartName: "test/chart",
										Version:   "1.0.0",
										Values:    "abc: def",
										TargetNS:  "test-ns",
										Order:     1,
									},
								},
							},
						},
					},
				},
			},
			in: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{
							Helm: &ecv1beta1.Helm{
								Charts: []ecv1beta1.Chart{
									{
										Name:      "test-chart",
										ChartName: "test/chart",
										Version:   "2.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns",
										Order:     1,
									},
								},
							},
						},
					},
				},
			},
			setupMockHelmCli: func(t *testing.T) *helm.MockClient {
				helmCli := &helm.MockClient{}
				mock.InOrder(
					helmCli.
						On("ReleaseExists", mock.Anything, "test-ns", "test-chart").
						Once().
						Return(true, nil),
					helmCli.
						On("Upgrade", mock.Anything, helm.UpgradeOptions{
							ReleaseName:  "test-chart",
							ChartPath:    "test/chart",
							ChartVersion: "2.0.0",
							Values:       map[string]interface{}{"abc": "xyz"},
							Namespace:    "test-ns",
							Force:        true,
						}).
						Once().
						Return(nil, nil),
				)
				return helmCli
			},
			validateIn: func(t *testing.T, in *ecv1beta1.Installation) {
				assert.Len(t, in.Status.Conditions, 1, "expected 1 condition")
				assert.Equal(t, "test-ns-test-chart", in.Status.Conditions[0].Type, "expected condition type")
				assert.Equal(t, metav1.ConditionTrue, in.Status.Conditions[0].Status, "expected condition status")
				assert.Equal(t, "Upgraded", in.Status.Conditions[0].Reason, "expected condition reason")
			},
			wantErr: false,
		},
		{
			name: "install if release does not exist during upgrade",
			prev: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{
							Helm: &ecv1beta1.Helm{
								Charts: []ecv1beta1.Chart{
									{
										Name:      "test-chart",
										ChartName: "test/chart",
										Version:   "1.0.0",
										Values:    "abc: def",
										TargetNS:  "test-ns",
										Order:     1,
									},
								},
							},
						},
					},
				},
			},
			in: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{
							Helm: &ecv1beta1.Helm{
								Charts: []ecv1beta1.Chart{
									{
										Name:      "test-chart",
										ChartName: "test/chart",
										Version:   "2.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns",
										Order:     1,
									},
								},
							},
						},
					},
				},
			},
			setupMockHelmCli: func(t *testing.T) *helm.MockClient {
				helmCli := &helm.MockClient{}
				mock.InOrder(
					helmCli.
						On("ReleaseExists", mock.Anything, "test-ns", "test-chart").
						Once().
						Return(false, nil),
					helmCli.
						On("Install", mock.Anything, helm.InstallOptions{
							ReleaseName:  "test-chart",
							ChartPath:    "test/chart",
							ChartVersion: "2.0.0",
							Values:       map[string]interface{}{"abc": "xyz"},
							Namespace:    "test-ns",
						}).
						Once().
						Return(nil, nil),
				)
				return helmCli
			},
			validateIn: func(t *testing.T, in *ecv1beta1.Installation) {
				assert.Len(t, in.Status.Conditions, 1, "expected 1 condition")
				assert.Equal(t, "test-ns-test-chart", in.Status.Conditions[0].Type, "expected condition type")
				assert.Equal(t, metav1.ConditionTrue, in.Status.Conditions[0].Status, "expected condition status")
				assert.Equal(t, "Upgraded", in.Status.Conditions[0].Reason, "expected condition reason")
			},
			wantErr: false,
		},
		{
			name: "skip upgrade if no changes",
			prev: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{
							Helm: &ecv1beta1.Helm{
								Charts: []ecv1beta1.Chart{
									{
										Name:      "test-chart",
										ChartName: "test/chart",
										Version:   "1.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns",
										Order:     1,
									},
								},
							},
						},
					},
				},
			},
			in: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{
							Helm: &ecv1beta1.Helm{
								Charts: []ecv1beta1.Chart{
									{
										Name:      "test-chart",
										ChartName: "test/chart",
										Version:   "1.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns",
										Order:     1,
									},
								},
							},
						},
					},
				},
			},
			setupMockHelmCli: func(t *testing.T) *helm.MockClient {
				helmCli := &helm.MockClient{}
				// No helm client calls should be made since there are no changes
				return helmCli
			},
			validateIn: func(t *testing.T, in *ecv1beta1.Installation) {
				assert.Len(t, in.Status.Conditions, 1, "expected 1 condition")
				assert.Equal(t, "test-ns-test-chart", in.Status.Conditions[0].Type, "expected condition type")
				assert.Equal(t, metav1.ConditionTrue, in.Status.Conditions[0].Status, "expected condition status")
				assert.Equal(t, "Upgraded", in.Status.Conditions[0].Reason, "expected condition reason")
			},
			wantErr: false,
		},
		{
			name: "remove if release exists",
			prev: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{
							Helm: &ecv1beta1.Helm{
								Charts: []ecv1beta1.Chart{
									{
										Name:      "to-remove-chart",
										ChartName: "test/chart",
										Version:   "1.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns",
										Order:     1,
									},
								},
							},
						},
					},
				},
			},
			in: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{},
					},
				},
			},
			setupMockHelmCli: func(t *testing.T) *helm.MockClient {
				helmCli := &helm.MockClient{}
				mock.InOrder(
					helmCli.
						On("ReleaseExists", mock.Anything, "test-ns", "to-remove-chart").
						Once().
						Return(true, nil),
					helmCli.
						On("Uninstall", mock.Anything, helm.UninstallOptions{
							ReleaseName: "to-remove-chart",
							Namespace:   "test-ns",
							Wait:        true,
						}).
						Once().
						Return(nil),
				)
				return helmCli
			},
			validateIn: func(t *testing.T, in *ecv1beta1.Installation) {
				assert.Len(t, in.Status.Conditions, 1, "expected 1 condition")
				assert.Equal(t, "test-ns-to-remove-chart", in.Status.Conditions[0].Type, "expected condition type")
				assert.Equal(t, metav1.ConditionTrue, in.Status.Conditions[0].Status, "expected condition status")
				assert.Equal(t, "Uninstalled", in.Status.Conditions[0].Reason, "expected condition reason")
			},
			wantErr: false,
		},
		{
			name: "skip remove if release does not exist",
			prev: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{
							Helm: &ecv1beta1.Helm{
								Charts: []ecv1beta1.Chart{
									{
										Name:      "nonexistent-chart",
										ChartName: "test/chart",
										Version:   "1.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns",
										Order:     1,
									},
								},
							},
						},
					},
				},
			},
			in: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{},
					},
				},
			},
			setupMockHelmCli: func(t *testing.T) *helm.MockClient {
				helmCli := &helm.MockClient{}
				mock.InOrder(
					helmCli.
						On("ReleaseExists", mock.Anything, "test-ns", "nonexistent-chart").
						Once().
						Return(false, nil),
				)
				return helmCli
			},
			validateIn: func(t *testing.T, in *ecv1beta1.Installation) {
				assert.Len(t, in.Status.Conditions, 1, "expected 1 condition")
				assert.Equal(t, "test-ns-nonexistent-chart", in.Status.Conditions[0].Type, "expected condition type")
				assert.Equal(t, metav1.ConditionTrue, in.Status.Conditions[0].Status, "expected condition status")
				assert.Equal(t, "Uninstalled", in.Status.Conditions[0].Reason, "expected condition reason")
			},
			wantErr: false,
		},
		{
			name: "adding, upgrading and removing extensions happens in the correct order",
			prev: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{
							Helm: &ecv1beta1.Helm{
								Charts: []ecv1beta1.Chart{
									{
										Name:      "test-upgrade1",
										ChartName: "test/upgrade1",
										Version:   "1.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns1",
										Order:     2,
									},
									{
										Name:      "test-old1",
										ChartName: "test/old1",
										Version:   "1.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns1",
										Order:     3,
									},
									{
										Name:      "test-old2",
										ChartName: "test/old2",
										Version:   "2.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns2",
										Order:     4,
									},
								},
							},
						},
					},
				},
			},
			in: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Extensions: ecv1beta1.Extensions{
							Helm: &ecv1beta1.Helm{
								Charts: []ecv1beta1.Chart{
									{
										Name:      "test-new1",
										ChartName: "test/new1",
										Version:   "1.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns1",
										Order:     10,
									},
									{
										Name:      "test-new2",
										ChartName: "test/new2",
										Version:   "2.0.0",
										Values:    "abc: xyz",
										TargetNS:  "test-ns2",
										Order:     1,
									},
									{
										Name:         "test-upgrade1",
										ChartName:    "test/upgrade1",
										Version:      "1.0.0",
										Values:       "abc: xyz",
										TargetNS:     "test-ns1",
										Order:        2,
										ForceUpgrade: ptr.To(false),
									},
								},
							},
						},
					},
				},
			},
			setupMockHelmCli: func(t *testing.T) *helm.MockClient {
				helmCli := &helm.MockClient{}
				// Setup expectations for helm client
				mockCtx := mock.Anything

				mock.InOrder(
					// remove happens first in reverse order
					helmCli.
						On("ReleaseExists", mockCtx, "test-ns2", "test-old2").
						Once().
						Return(true, nil),
					helmCli.
						On("Uninstall", mockCtx, helm.UninstallOptions{
							ReleaseName: "test-old2",
							Namespace:   "test-ns2",
							Wait:        true,
						}).
						Once().
						Return(nil),
					helmCli.
						On("ReleaseExists", mockCtx, "test-ns1", "test-old1").
						Once().
						Return(true, nil),
					helmCli.
						On("Uninstall", mockCtx, helm.UninstallOptions{
							ReleaseName: "test-old1",
							Namespace:   "test-ns1",
							Wait:        true,
						}).
						Once().
						Return(nil),

					// install and upgrade happens together in order
					helmCli.
						On("ReleaseExists", mockCtx, "test-ns2", "test-new2").
						Once().
						Return(false, nil),
					helmCli.
						On("Install", mockCtx, helm.InstallOptions{
							ReleaseName:  "test-new2",
							ChartPath:    "test/new2",
							ChartVersion: "2.0.0",
							Values:       map[string]interface{}{"abc": "xyz"},
							Namespace:    "test-ns2",
						}).
						Once().
						Return(nil, nil),
					helmCli.
						On("ReleaseExists", mockCtx, "test-ns1", "test-upgrade1").
						Once().
						Return(true, nil),
					helmCli.On("Upgrade", mockCtx, helm.UpgradeOptions{
						ReleaseName:  "test-upgrade1",
						ChartPath:    "test/upgrade1",
						ChartVersion: "1.0.0",
						Values:       map[string]interface{}{"abc": "xyz"},
						Namespace:    "test-ns1",
						Force:        false,
					}).
						Once().
						Return(nil, nil),
					helmCli.
						On("ReleaseExists", mockCtx, "test-ns1", "test-new1").
						Once().
						Return(false, nil),
					helmCli.
						On("Install", mockCtx, helm.InstallOptions{
							ReleaseName:  "test-new1",
							ChartPath:    "test/new1",
							ChartVersion: "1.0.0",
							Values:       map[string]interface{}{"abc": "xyz"},
							Namespace:    "test-ns1",
						}).
						Once().
						Return(nil, nil),
				)
				return helmCli
			},
			validateIn: func(t *testing.T, in *ecv1beta1.Installation) {
				assert.Len(t, in.Status.Conditions, 5, "expected 5 conditions")
				for _, cond := range in.Status.Conditions {
					assert.Equal(t, metav1.ConditionTrue, cond.Status, "expected condition status")
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.in.Name = "test-installation"

			kcli := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&ecv1beta1.Installation{}).
				WithObjects(tt.in).
				Build()
			mockHelmCli := tt.setupMockHelmCli(t)

			err := Upgrade(context.Background(), kcli, mockHelmCli, tt.prev, tt.in, logger)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify that all expected calls were made
			mockHelmCli.AssertExpectations(t)

			if tt.validateIn != nil {
				tt.validateIn(t, tt.in)
			}
		})
	}
}
