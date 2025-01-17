package migratev2

import (
	"context"
	"encoding/json"
	"fmt"
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

func Test_runManagerInstallJobsAndWait(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a goroutine to simulate the jobs completing successfully
	go func() {
		for {
			time.Sleep(100 * time.Millisecond) // Give time for jobs to be created

			// List the jobs
			var jobs batchv1.JobList
			err := cli.List(ctx, &jobs)
			require.NoError(t, err)

			// Update each job to be successful
			for _, job := range jobs.Items {
				// Update job status
				job.Status.Succeeded = 1
				fmt.Printf("Updating job %s to be successful\n", job.Name)
				err = cli.Status().Update(ctx, &job)
				require.NoError(t, err)
			}

			if len(jobs.Items) == len(nodes) {
				break
			}
		}
	}()

	// Run the function
	logf := func(format string, args ...any) {
		// No-op logger for testing
	}

	err := runManagerInstallJobsAndWait(ctx, logf, cli, installation, "test-secret", "test-app", "v1.0.0")
	require.NoError(t, err)

	// Verify the jobs were created and completed
	var jobs batchv1.JobList
	err = cli.List(ctx, &jobs)
	require.NoError(t, err)

	// Verify number of jobs matches number of nodes
	assert.Len(t, jobs.Items, len(nodes))

	// Verify each job
	for _, job := range jobs.Items {
		// Verify job succeeded
		assert.Equal(t, int32(1), job.Status.Succeeded)

		// Verify job has correct labels
		assert.Equal(t, "install-v2-manager", job.Labels["app"])

		// Verify job spec
		podSpec := job.Spec.Template.Spec
		assert.Equal(t, "install-v2-manager", podSpec.Containers[0].Name)
		assert.Equal(t, corev1.RestartPolicyOnFailure, podSpec.RestartPolicy)

		// Verify node affinity
		if job.Name == "install-v2-manager-node1" {
			assert.Equal(t, "node1", podSpec.NodeSelector["kubernetes.io/hostname"])

			// Verify tolerations are set for tainted nodes
			expectedToleration := corev1.Toleration{
				Key:      "node-role.kubernetes.io/control-plane",
				Value:    "",
				Operator: corev1.TolerationOpEqual,
			}
			assert.Contains(t, podSpec.Tolerations, expectedToleration)
		} else {
			assert.Equal(t, "node2", podSpec.NodeSelector["kubernetes.io/hostname"])
		}

		// Verify volumes
		foundInstallationVolume := false
		foundLicenseVolume := false
		for _, volume := range podSpec.Volumes {
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
		container := podSpec.Containers[0]
		assert.Equal(t, container.Image, "embedded-cluster-operator-image:1.0")
		assert.Contains(t, container.Command, "migrate-v2")
		assert.Contains(t, container.Command, "install-manager")
		assert.Contains(t, container.Command, "--app-slug")
		assert.Contains(t, container.Command, "test-app")
		assert.Contains(t, container.Command, "--app-version-label")
		assert.Contains(t, container.Command, "v1.0.0")
	}
}

func Test_deleteManagerInstallJobs(t *testing.T) {
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

	// Create existing jobs
	jobs := []batchv1.Job{
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

	// Create fake client with nodes and jobs
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(append(
			nodesToRuntimeObjects(nodes),
			jobsToRuntimeObjects(jobs)...,
		)...).
		Build()

	// Run the function
	logf := func(format string, args ...any) {
		// No-op logger for testing
	}

	err := deleteManagerInstallJobs(context.Background(), logf, cli)
	require.NoError(t, err)

	// Verify jobs were deleted
	var remainingJobs batchv1.JobList
	err = cli.List(context.Background(), &remainingJobs)
	require.NoError(t, err)
	assert.Empty(t, remainingJobs.Items, "expected all jobs to be deleted")
}

func nodesToRuntimeObjects(nodes []corev1.Node) []client.Object {
	objects := make([]client.Object, len(nodes))
	for i := range nodes {
		objects[i] = &nodes[i]
	}
	return objects
}

func Test_ensureManagerInstallJobForNode(t *testing.T) {
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
		existingJob  *batchv1.Job
		expectNewJob bool
		validateJob  func(*testing.T, *batchv1.Job)
		expectError  bool
	}{
		{
			name:        "creates new job when none exists",
			existingJob: nil,
			validateJob: func(t *testing.T, job *batchv1.Job) {
				assert.Equal(t, "install-v2-manager-test-node", job.Name)
				assert.Equal(t, "embedded-cluster", job.Namespace)
				assert.Equal(t, "install-v2-manager", job.Labels["app"])

				podSpec := job.Spec.Template.Spec
				assert.Equal(t, "test-node", podSpec.NodeSelector["kubernetes.io/hostname"])

				// Verify tolerations
				expectedToleration := corev1.Toleration{
					Key:      "node-role.kubernetes.io/control-plane",
					Value:    "",
					Operator: corev1.TolerationOpEqual,
				}
				assert.Contains(t, podSpec.Tolerations, expectedToleration)

				// Verify volumes
				foundInstallationVolume := false
				foundLicenseVolume := false
				for _, volume := range podSpec.Volumes {
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
				container := podSpec.Containers[0]
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
			name: "reuses existing successful job",
			existingJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "install-v2-manager-test-node",
					Namespace: "embedded-cluster",
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
				},
			},
			validateJob: func(t *testing.T, job *batchv1.Job) {
				assert.Equal(t, int32(1), job.Status.Succeeded)
			},
			expectError: false,
		},
		{
			name: "replaces failed job",
			existingJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "install-v2-manager-test-node",
					Namespace: "embedded-cluster",
				},
				Status: batchv1.JobStatus{
					Failed: 1,
				},
			},
			validateJob: func(t *testing.T, job *batchv1.Job) {
				assert.Equal(t, int32(0), job.Status.Failed)
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
			if tt.existingJob != nil {
				builder = builder.WithObjects(tt.existingJob)
			}
			cli := builder.Build()

			// Run the function
			jobName, err := ensureManagerInstallJobForNode(
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

			// Get the job
			var job batchv1.Job
			err = cli.Get(context.Background(), client.ObjectKey{
				Namespace: "embedded-cluster",
				Name:      jobName,
			}, &job)
			require.NoError(t, err)

			// Run validation
			tt.validateJob(t, &job)
		})
	}
}

func jobsToRuntimeObjects(jobs []batchv1.Job) []client.Object {
	objects := make([]client.Object, len(jobs))
	for i := range jobs {
		objects[i] = &jobs[i]
	}
	return objects
}
