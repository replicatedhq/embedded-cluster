package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_runMigrateV2PodAndWait(t *testing.T) {
	// Start a mock server for the metadata request
	metadataServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metadata := map[string]interface{}{
			"images": []string{
				"embedded-cluster-operator-image:1.0",
			},
		}
		json.NewEncoder(w).Encode(metadata)
	}))
	defer metadataServer.Close()

	// Create test installation with metadata override URL
	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-install",
		},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version:             "1.0.0",
				MetadataOverrideURL: metadataServer.URL,
			},
		},
	}

	// Set up the test scheme
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, ecv1beta1.AddToScheme(scheme))

	// Create fake client with nodes
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation).
		Build()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start a goroutine to simulate the pod completing successfully
	go func() {
		for {
			time.Sleep(100 * time.Millisecond) // Give time for pod to be created

			// Get the pod
			nsn := apitypes.NamespacedName{Namespace: migrateV2PodNamespace, Name: migrateV2PodName}
			var pod corev1.Pod
			err := cli.Get(ctx, nsn, &pod)
			if k8serrors.IsNotFound(err) {
				continue
			}
			require.NoError(t, err)

			// Update the pod to be successful
			// Update pod status
			pod.Status.Phase = corev1.PodSucceeded
			err = cli.Status().Update(ctx, &pod)
			require.NoError(t, err)
		}
	}()

	// Run the function
	logf := func(format string, args ...any) {
		// No-op logger for testing
	}

	err := runMigrateV2PodAndWait(ctx, logf, cli, installation, "test-secret", "test-app", "v1.0.0")
	require.NoError(t, err)

	// Verify the pod was created and completed
	nsn := apitypes.NamespacedName{Namespace: migrateV2PodNamespace, Name: migrateV2PodName}
	var pod corev1.Pod
	err = cli.Get(ctx, nsn, &pod)
	require.NoError(t, err)

	// Verify pod succeeded
	assert.Equal(t, corev1.PodSucceeded, pod.Status.Phase)

	// Verify pod has correct labels
	assert.Equal(t, "install-v2-manager", pod.Labels["app"])

	// Verify pod spec
	assert.Equal(t, "install-v2-manager", pod.Spec.Containers[0].Name)
	assert.Equal(t, corev1.RestartPolicyNever, pod.Spec.RestartPolicy)

	// Verify volumes
	foundInstallationVolume := false
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == "installation" {
			foundInstallationVolume = true
			assert.Equal(t, "test-install", volume.ConfigMap.Name)
		}
	}
	assert.True(t, foundInstallationVolume, "expected installation volume to be mounted")

	// Verify command arguments
	container := pod.Spec.Containers[0]
	assert.Equal(t, container.Image, "embedded-cluster-operator-image:1.0")
	assert.Contains(t, container.Command, "migrate-v2")
	assert.Contains(t, container.Command, "--migrate-v2-secret")
	assert.Contains(t, container.Command, "test-secret")
	assert.Contains(t, container.Command, "--app-slug")
	assert.Contains(t, container.Command, "test-app")
	assert.Contains(t, container.Command, "--app-version-label")
	assert.Contains(t, container.Command, "v1.0.0")
}

func Test_deleteMigrateV2Pod(t *testing.T) {
	// Create existing pod
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      migrateV2PodName,
				Namespace: migrateV2PodNamespace,
			},
		},
	}

	// Set up the test scheme
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))

	// Create fake client with nodes and pods
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			podsToRuntimeObjects(pods)...,
		).
		Build()

	// Run the function
	logf := func(format string, args ...any) {
		// No-op logger for testing
	}

	err := deleteMigrateV2Pod(context.Background(), logf, cli)
	require.NoError(t, err)

	// Verify pods were deleted
	var remainingPods corev1.PodList
	err = cli.List(context.Background(), &remainingPods)
	require.NoError(t, err)
	assert.Empty(t, remainingPods.Items, "expected all pods to be deleted")
}

func nodesToRuntimeObjects(nodes []corev1.Node) []client.Object {
	objects := make([]client.Object, len(nodes))
	for i := range nodes {
		objects[i] = &nodes[i]
	}
	return objects
}

func Test_ensureMigrateV2Pod(t *testing.T) {
	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-install",
		},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "1.0.0",
			},
		},
	}

	tests := []struct {
		name         string
		existingPod  *corev1.Pod
		expectNewPod bool
		validatePod  func(*testing.T, *corev1.Pod)
		expectError  bool
	}{
		{
			name:        "creates new pod when none exists",
			existingPod: nil,
			validatePod: func(t *testing.T, pod *corev1.Pod) {
				assert.Equal(t, migrateV2PodName, pod.Name)
				assert.Equal(t, migrateV2PodNamespace, pod.Namespace)

				// Verify volumes
				foundInstallationVolume := false
				for _, volume := range pod.Spec.Volumes {
					if volume.Name == "installation" {
						foundInstallationVolume = true
						assert.Equal(t, "test-install", volume.ConfigMap.Name)
					}
				}
				assert.True(t, foundInstallationVolume, "expected installation volume to be mounted")

				// Verify container
				container := pod.Spec.Containers[0]
				assert.Equal(t, container.Image, "embedded-cluster-operator-image:1.0")
				assert.Contains(t, container.Command, "migrate-v2")
				assert.Contains(t, container.Command, "--migrate-v2-secret")
				assert.Contains(t, container.Command, "test-secret")
				assert.Contains(t, container.Command, "--app-slug")
				assert.Contains(t, container.Command, "test-app")
				assert.Contains(t, container.Command, "--app-version-label")
				assert.Contains(t, container.Command, "v1.0.0")
			},
			expectError: false,
		},
		{
			name: "reuses existing successful pod",
			existingPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      migrateV2PodName,
					Namespace: migrateV2PodNamespace,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodSucceeded,
				},
			},
			validatePod: func(t *testing.T, pod *corev1.Pod) {
				assert.Equal(t, corev1.PodSucceeded, pod.Status.Phase)
			},
			expectError: false,
		},
		{
			name: "replaces failed pod",
			existingPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      migrateV2PodName,
					Namespace: migrateV2PodNamespace,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
				},
			},
			validatePod: func(t *testing.T, pod *corev1.Pod) {
				assert.Equal(t, corev1.PodPhase(""), pod.Status.Phase)
			},
			expectError: false,
		},
	}

	// Set up the test scheme
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, ecv1beta1.AddToScheme(scheme))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Create fake client
			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.existingPod != nil {
				builder = builder.WithObjects(tt.existingPod)
			}
			cli := builder.Build()

			// Run the function
			podName, err := ensureMigrateV2Pod(
				context.Background(),
				cli,
				installation,
				"embedded-cluster-operator-image:1.0",
				"test-secret",
				"test-app",
				"v1.0.0",
			)

			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Get the pod
			var pod corev1.Pod
			err = cli.Get(context.Background(), client.ObjectKey{
				Namespace: migrateV2PodNamespace,
				Name:      podName,
			}, &pod)
			require.NoError(t, err)

			// Run validation
			tt.validatePod(t, &pod)
		})
	}
}

func podsToRuntimeObjects(pods []corev1.Pod) []client.Object {
	objects := make([]client.Object, len(pods))
	for i := range pods {
		objects[i] = &pods[i]
	}
	return objects
}
