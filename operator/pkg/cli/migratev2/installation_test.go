package migratev2

import (
	"context"
	"testing"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_waitForInstallationStateInstalled(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))

	tests := []struct {
		name         string
		installation *ecv1beta1.Installation
		updateFunc   func(*ecv1beta1.Installation) // Function to update installation state during test
		expectError  bool
		errorString  string
	}{
		{
			name: "installation already in installed state",
			installation: &ecv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-installation",
				},
				Status: ecv1beta1.InstallationStatus{
					State: ecv1beta1.InstallationStateInstalled,
				},
			},
			expectError: false,
		},
		{
			name: "installation already in failed state",
			installation: &ecv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-installation",
				},
				Status: ecv1beta1.InstallationStatus{
					State:  ecv1beta1.InstallationStateFailed,
					Reason: "something went wrong",
				},
			},
			expectError: true,
			errorString: "installation failed: something went wrong",
		},
		{
			name: "installation transitions to installed state",
			installation: &ecv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-installation",
				},
				Status: ecv1beta1.InstallationStatus{
					State: ecv1beta1.InstallationStateAddonsInstalling,
				},
			},
			updateFunc: func(in *ecv1beta1.Installation) {
				in.Status.State = ecv1beta1.InstallationStateInstalled
			},
			expectError: false,
		},
		{
			name: "installation transitions to failed state",
			installation: &ecv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-installation",
				},
				Status: ecv1beta1.InstallationStatus{
					State: ecv1beta1.InstallationStateAddonsInstalling,
				},
			},
			updateFunc: func(in *ecv1beta1.Installation) {
				in.Status.State = ecv1beta1.InstallationStateHelmChartUpdateFailure
				in.Status.Reason = "helm chart update failed"
			},
			expectError: true,
			errorString: "installation failed: helm chart update failed",
		},
		{
			name: "installation becomes obsolete",
			installation: &ecv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-installation",
				},
				Status: ecv1beta1.InstallationStatus{
					State: ecv1beta1.InstallationStateAddonsInstalling,
				},
			},
			updateFunc: func(in *ecv1beta1.Installation) {
				in.Status.State = ecv1beta1.InstallationStateObsolete
				in.Status.Reason = "This is not the most recent installation object"
			},
			expectError: true,
			errorString: "installation is obsolete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake client with the test installation
			cli := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.installation).
				WithStatusSubresource(tt.installation).
				Build()

			// If there's an update function, run it in a goroutine after a short delay
			if tt.updateFunc != nil {
				go func() {
					time.Sleep(100 * time.Millisecond)
					var installation ecv1beta1.Installation
					err := cli.Get(context.Background(), client.ObjectKey{Name: tt.installation.Name}, &installation)
					require.NoError(t, err)
					tt.updateFunc(&installation)
					err = cli.Status().Update(context.Background(), &installation)
					require.NoError(t, err)
				}()
			}

			// Call waitForInstallationStateInstalled
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err := waitForInstallationStateInstalled(ctx, t.Logf, cli, tt.installation)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorString != "" {
					assert.Contains(t, err.Error(), tt.errorString)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_ensureInstallationStateInstalled(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	tests := []struct {
		name         string
		installation *ecv1beta1.Installation
		expectError  bool
	}{
		{
			name: "updates installation state and removes conditions",
			installation: &ecv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-installation",
				},
				Status: ecv1beta1.InstallationStatus{
					State: ecv1beta1.InstallationStateAddonsInstalled,
					Conditions: []metav1.Condition{
						{
							Type:               ecv1beta1.ConditionTypeV2MigrationInProgress,
							Status:             metav1.ConditionTrue,
							Reason:             "V2MigrationInProgress",
							ObservedGeneration: 1,
						},
						{
							Type:               ecv1beta1.ConditionTypeDisableReconcile,
							Status:             metav1.ConditionTrue,
							Reason:             "V2MigrationInProgress",
							ObservedGeneration: 1,
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake client with the test installation
			cli := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.installation).
				WithStatusSubresource(tt.installation).
				Build()

			// Call ensureInstallationStateInstalled
			err := ensureInstallationStateInstalled(context.Background(), t.Logf, cli, tt.installation)

			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify the installation was updated correctly
			var updatedInstallation ecv1beta1.Installation
			err = cli.Get(context.Background(), client.ObjectKey{Name: tt.installation.Name}, &updatedInstallation)
			require.NoError(t, err)

			// Check state is set to Installed
			assert.Equal(t, ecv1beta1.InstallationStateInstalled, updatedInstallation.Status.State)
			assert.Equal(t, "V2MigrationComplete", updatedInstallation.Status.Reason)

			// Check conditions are removed
			condition := meta.FindStatusCondition(updatedInstallation.Status.Conditions, ecv1beta1.ConditionTypeV2MigrationInProgress)
			assert.Nil(t, condition, "V2MigrationInProgress condition should be removed")

			condition = meta.FindStatusCondition(updatedInstallation.Status.Conditions, ecv1beta1.ConditionTypeDisableReconcile)
			assert.Nil(t, condition, "DisableReconcile condition should be removed")
		})
	}
}
