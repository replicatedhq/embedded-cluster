package migratev2

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_runManagerInstallPodsAndWait(t *testing.T) {
	// Create test nodes, one with taints
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{
						Key:    "node-role.kubernetes.io/control-plane",
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node2",
			},
		},
	}

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
		WithObjects(nodesToRuntimeObjects(nodes)...).
		WithObjects(installation).
		Build()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start a goroutine to simulate the pods completing successfully
	go func() {
		for {
			time.Sleep(100 * time.Millisecond) // Give time for pods to be created

			// List the pods
			var pods corev1.PodList
			err := cli.List(ctx, &pods)
			require.NoError(t, err)

			// Update each pod to be successful
			for _, pod := range pods.Items {
				// Update pod status
				pod.Status.Phase = corev1.PodSucceeded
				err = cli.Status().Update(ctx, &pod)
				require.NoError(t, err)
			}

			if len(pods.Items) == len(nodes) {
				break
			}
		}
	}()

	// Run the function
	logf := func(format string, args ...any) {
		// No-op logger for testing
	}

	err := runManagerInstallPodsAndWait(ctx, logf, cli, installation, "test-secret", "test-app", "v1.0.0")
	require.NoError(t, err)

	// Verify the pods were created and completed
	var pods corev1.PodList
	err = cli.List(ctx, &pods)
	require.NoError(t, err)

	// Verify number of pods matches number of nodes
	assert.Len(t, pods.Items, len(nodes))

	// Verify each pod
	for _, pod := range pods.Items {
		// Verify pod succeeded
		assert.Equal(t, corev1.PodSucceeded, pod.Status.Phase)

		// Verify pod has correct labels
		assert.Equal(t, "install-v2-manager", pod.Labels["app"])

		// Verify pod spec
		assert.Equal(t, "install-v2-manager", pod.Spec.Containers[0].Name)
		assert.Equal(t, corev1.RestartPolicyNever, pod.Spec.RestartPolicy)

		// Verify node affinity
		if pod.Name == "install-v2-manager-node1" {
			assert.Equal(t, "node1", pod.Spec.NodeSelector["kubernetes.io/hostname"])

			// Verify tolerations are set for tainted nodes
			expectedToleration := corev1.Toleration{
				Key:      "node-role.kubernetes.io/control-plane",
				Value:    "",
				Operator: corev1.TolerationOpEqual,
			}
			assert.Contains(t, pod.Spec.Tolerations, expectedToleration)
		} else {
			assert.Equal(t, "node2", pod.Spec.NodeSelector["kubernetes.io/hostname"])
		}

		// Verify volumes
		foundInstallationVolume := false
		foundLicenseVolume := false
		for _, volume := range pod.Spec.Volumes {
			if volume.Name == "installation" {
				foundInstallationVolume = true
				assert.Equal(t, "test-install", volume.ConfigMap.Name)
			}
			if volume.Name == "license" {
				foundLicenseVolume = true
				assert.Equal(t, "test-secret", volume.Secret.SecretName)
			}
		}
		assert.True(t, foundInstallationVolume, "expected installation volume to be mounted")
		assert.True(t, foundLicenseVolume, "expected license volume to be mounted")

		// Verify command arguments
		container := pod.Spec.Containers[0]
		assert.Equal(t, container.Image, "embedded-cluster-operator-image:1.0")
		assert.Contains(t, container.Command, "migrate-v2")
		assert.Contains(t, container.Command, "install-manager")
		assert.Contains(t, container.Command, "--app-slug")
		assert.Contains(t, container.Command, "test-app")
		assert.Contains(t, container.Command, "--app-version-label")
		assert.Contains(t, container.Command, "v1.0.0")
	}
}

func Test_deleteManagerInstallPods(t *testing.T) {
	// Create test nodes
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node2",
			},
		},
	}

	// Create existing pods
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "install-v2-manager-node1",
				Namespace: "embedded-cluster",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "install-v2-manager-node2",
				Namespace: "embedded-cluster",
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
		WithObjects(append(
			nodesToRuntimeObjects(nodes),
			podsToRuntimeObjects(pods)...,
		)...).
		Build()

	// Run the function
	logf := func(format string, args ...any) {
		// No-op logger for testing
	}

	err := deleteManagerInstallPods(context.Background(), logf, cli)
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

func Test_ensureManagerInstallPodForNode(t *testing.T) {
	// Set up common test objects
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    "node-role.kubernetes.io/control-plane",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
	}

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
				assert.Equal(t, "install-v2-manager-test-node", pod.Name)
				assert.Equal(t, "embedded-cluster", pod.Namespace)
				assert.Equal(t, "install-v2-manager", pod.Labels["app"])

				assert.Equal(t, "test-node", pod.Spec.NodeSelector["kubernetes.io/hostname"])

				// Verify tolerations
				expectedToleration := corev1.Toleration{
					Key:      "node-role.kubernetes.io/control-plane",
					Value:    "",
					Operator: corev1.TolerationOpEqual,
				}
				assert.Contains(t, pod.Spec.Tolerations, expectedToleration)

				// Verify volumes
				foundInstallationVolume := false
				foundLicenseVolume := false
				for _, volume := range pod.Spec.Volumes {
					if volume.Name == "installation" {
						foundInstallationVolume = true
						assert.Equal(t, "test-install", volume.ConfigMap.Name)
					}
					if volume.Name == "license" {
						foundLicenseVolume = true
						assert.Equal(t, "test-secret", volume.Secret.SecretName)
					}
				}
				assert.True(t, foundInstallationVolume, "expected installation volume to be mounted")
				assert.True(t, foundLicenseVolume, "expected license volume to be mounted")

				// Verify container
				container := pod.Spec.Containers[0]
				assert.Equal(t, "embedded-cluster-operator-image:1.0", container.Image)
				assert.Contains(t, container.Command, "migrate-v2")
				assert.Contains(t, container.Command, "install-manager")
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
					Name:      "install-v2-manager-test-node",
					Namespace: "embedded-cluster",
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
					Name:      "install-v2-manager-test-node",
					Namespace: "embedded-cluster",
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
			podName, err := ensureManagerInstallPodForNode(
				context.Background(),
				cli,
				node,
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
				Namespace: "embedded-cluster",
				Name:      podName,
			}, &pod)
			require.NoError(t, err)

			// Run validation
			tt.validatePod(t, &pod)
		})
	}
}
