package openebs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestOpenEBS_ensurePreUpgradeHooksDeleted(t *testing.T) {
	tests := []struct {
		name       string
		objects    []client.Object
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "successful deletion of all resources",
			objects: []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "openebs-pre-upgrade-hook",
						Namespace: "openebs",
					},
				},
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "openebs-pre-upgrade-hook",
					},
				},
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "openebs-pre-upgrade-hook",
					},
				},
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "openebs-pre-upgrade-hook",
						Namespace: "openebs",
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "all resources not found - should succeed",
			objects: []client.Object{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with objects
			fakeClient := fake.NewClientBuilder().
				WithObjects(tt.objects...).
				Build()

			// Create OpenEBS instance
			openebs := &OpenEBS{}

			// Call the function
			err := openebs.ensurePreUpgradeHooksDeleted(context.Background(), fakeClient)

			// Verify results
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			} else {
				require.NoError(t, err)
			}

			// Verify that objects were deleted (for successful deletion test)
			if len(tt.objects) > 0 {
				for _, obj := range tt.objects {
					err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)
					assert.True(t, apierrors.IsNotFound(err), "Object should be deleted: %v", obj)
				}
			}
		})
	}
}
