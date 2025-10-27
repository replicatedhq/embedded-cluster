package controllers

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/metadata"
	metadatafake "k8s.io/client-go/metadata/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInstallationReconciler_constructCreateCMCommand(t *testing.T) {
	in := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "install-name",
		},
		Spec: ecv1beta1.InstallationSpec{
			RuntimeConfig: &ecv1beta1.RuntimeConfigSpec{
				DataDir: "/var/lib/embedded-cluster",
			},
		},
	}

	rc := runtimeconfig.New(in.Spec.RuntimeConfig)

	job := constructHostPreflightResultsJob(rc, in, "my-node")

	require.Len(t, job.Spec.Template.Spec.Volumes, 2)
	require.Equal(t, "host", job.Spec.Template.Spec.Volumes[0].Name)
	require.Equal(t, "/var/lib/embedded-cluster", job.Spec.Template.Spec.Volumes[0].HostPath.Path)
	require.Len(t, job.Spec.Template.Spec.Containers, 1)
	require.Len(t, job.Spec.Template.Spec.Containers[0].Command, 4)
	kctlCmd := job.Spec.Template.Spec.Containers[0].Command[3]
	expected := "if [ -f /embedded-cluster/support/host-preflight-results.json ]; then /embedded-cluster/bin/kubectl create configmap ${HSPF_CM_NAME} --from-file=results.json=/embedded-cluster/support/host-preflight-results.json -n embedded-cluster --dry-run=client -oyaml | /embedded-cluster/bin/kubectl label -f - embedded-cluster/host-preflight-result=${EC_NODE_NAME} --local -o yaml | /embedded-cluster/bin/kubectl apply -f - && /embedded-cluster/bin/kubectl annotate configmap ${HSPF_CM_NAME} \"update-timestamp=$(date +'%Y-%m-%dT%H:%M:%SZ')\" --overwrite; else echo '/embedded-cluster/support/host-preflight-results.json does not exist'; fi"
	assert.Equal(t, expected, kctlCmd)
	require.Len(t, job.Spec.Template.Spec.Containers[0].Env, 3)
	assert.Equal(t, corev1.EnvVar{
		Name:  "EC_NODE_NAME",
		Value: "my-node",
	}, job.Spec.Template.Spec.Containers[0].Env[1])
	assert.Equal(t, corev1.EnvVar{
		Name:  "HSPF_CM_NAME",
		Value: "my-node-host-preflight-results",
	}, job.Spec.Template.Spec.Containers[0].Env[2])
}

func TestInstallationReconciler_reconcileHostCABundle(t *testing.T) {
	// Create a temporary file for testing CA bundle
	tempDir := t.TempDir()
	testCAPath := filepath.Join(tempDir, "test-ca.crt")
	err := helpers.WriteFile(testCAPath, []byte("new CA content"), 0644)
	require.NoError(t, err)

	namespace := "kotsadm"

	metascheme := metadatafake.NewTestScheme()
	metav1.AddMetaToScheme(metascheme)

	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	newConfigMap := func(content string) *corev1.ConfigMap {
		hash := md5.Sum([]byte(content))
		checksum := hex.EncodeToString(hash[:])
		return &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      adminconsole.PrivateCASConfigMapName,
				Namespace: namespace,
				Annotations: map[string]string{
					"replicated.com/cas-checksum": checksum,
				},
			},
			Data: map[string]string{
				"ca_0.crt": content,
			},
		}
	}

	tests := []struct {
		name               string
		caPath             string
		initClients        func(t *testing.T) (client.Client, metadata.Interface)
		expectedErr        bool
		expectedErrMessage string
	}{
		{
			name:   "should return nil when caPath is not set",
			caPath: "",
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				kcli := clientfake.NewClientBuilder().WithObjects(ns).Build()
				mcli := metadatafake.NewSimpleMetadataClient(metascheme)
				return kcli, mcli
			},
			expectedErr: false,
		},
		{
			name:   "should return nil when IsRequestEntityTooLargeError is returned from Create",
			caPath: testCAPath,
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				kcli := &mockClient{
					fake: clientfake.NewClientBuilder().WithObjects(ns).Build(),
					createFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						return &k8serrors.StatusError{
							ErrStatus: metav1.Status{
								Status:  metav1.StatusFailure,
								Message: "Request entity too large",
								Reason:  metav1.StatusReasonRequestEntityTooLarge,
								Code:    413,
							},
						}
					},
				}
				mcli := metadatafake.NewSimpleMetadataClient(metascheme)
				return kcli, mcli
			},
			expectedErr: false,
		},
		{
			name:   "should return nil when IsRequestEntityTooLargeError is returned from Patch",
			caPath: testCAPath,
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				cm := newConfigMap("old CA content")
				kcli := &mockClient{
					fake: clientfake.NewClientBuilder().WithObjects(ns, cm).Build(),
					patchFunc: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						return &k8serrors.StatusError{
							ErrStatus: metav1.Status{
								Status:  metav1.StatusFailure,
								Message: "Request entity too large",
								Reason:  metav1.StatusReasonRequestEntityTooLarge,
								Code:    413,
							},
						}
					},
				}
				mcli := metadatafake.NewSimpleMetadataClient(metascheme,
					&metav1.PartialObjectMetadata{TypeMeta: cm.TypeMeta, ObjectMeta: cm.ObjectMeta})
				return kcli, mcli
			},
			expectedErr: false,
		},
		{
			name:   "should return nil when IsNotExist is returned from reading CA file",
			caPath: filepath.Join(tempDir, "non-existent.crt"),
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				kcli := clientfake.NewClientBuilder().WithObjects(ns).Build()
				mcli := metadatafake.NewSimpleMetadataClient(metascheme)
				return kcli, mcli
			},
			expectedErr: false,
		},
		{
			name:   "should return error for other errors from Create",
			caPath: testCAPath,
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				kcli := &mockClient{
					fake: clientfake.NewClientBuilder().WithObjects(ns).Build(),
					createFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						return errors.New("some other create error")
					},
				}
				mcli := metadatafake.NewSimpleMetadataClient(metascheme)
				return kcli, mcli
			},
			expectedErr:        true,
			expectedErrMessage: "some other create error",
		},
		{
			name:   "should return error for other errors from Patch",
			caPath: testCAPath,
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				cm := newConfigMap("old CA content")
				kcli := &mockClient{
					fake: clientfake.NewClientBuilder().WithObjects(ns, cm).Build(),
					patchFunc: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						return errors.New("some other patch error")
					},
				}
				mcli := metadatafake.NewSimpleMetadataClient(metascheme,
					&metav1.PartialObjectMetadata{TypeMeta: cm.TypeMeta, ObjectMeta: cm.ObjectMeta})
				return kcli, mcli
			},
			expectedErr:        true,
			expectedErrMessage: "some other patch error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup reconciler with mock client
			scheme := runtime.NewScheme()
			// Register core v1 types to the scheme
			err := corev1.AddToScheme(scheme)
			require.NoError(t, err)

			kcli, mcli := tt.initClients(t)

			// Run test
			reconciler := &InstallationReconciler{
				Client:         kcli,
				MetadataClient: mcli,
				RuntimeConfig:  runtimeconfig.New(nil),
			}

			// Create a mock logger
			verbosity := 1
			if os.Getenv("DEBUG") != "" {
				verbosity = 10
			}
			logger := testr.NewWithOptions(t, testr.Options{Verbosity: verbosity})

			// Setup context with logger
			ctx := logr.NewContext(context.Background(), logger)

			t.Setenv("PRIVATE_CA_BUNDLE_PATH", tt.caPath)

			// Run test
			err = reconciler.reconcileHostCABundle(ctx)

			// Check results
			if tt.expectedErr {
				require.Error(t, err)
				if tt.expectedErrMessage != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMessage)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInstallationReconciler_deleteUpgradeJobs(t *testing.T) {
	// Create test namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kotsadm",
		},
	}

	// Helper function to create upgrade jobs
	createUpgradeJob := func(name, installationName string) *batchv1.Job {
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "kotsadm",
				Labels: map[string]string{
					"app.kubernetes.io/instance": "embedded-cluster-upgrade",
					"app.kubernetes.io/name":     "embedded-cluster-upgrade",
				},
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "upgrade",
								Image: "test-image",
							},
						},
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
		}
		if installationName != "" {
			if job.Annotations == nil {
				job.Annotations = make(map[string]string)
			}
			job.Annotations["embedded-cluster.replicated.com/installation-name"] = installationName
		}
		return job
	}

	// Create test installation
	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "install-b",
		},
	}

	tests := []struct {
		name           string
		installation   *ecv1beta1.Installation
		setupClient    func(t *testing.T) client.Client
		expectedErr    bool
		expectedErrMsg string
		validateFn     func(t *testing.T, cli client.Client)
	}{
		{
			name:         "successfully deletes all upgrade jobs",
			installation: installation,
			setupClient: func(t *testing.T) client.Client {
				job1 := createUpgradeJob("embedded-cluster-upgrade-install-1", "install-a")
				job2 := createUpgradeJob("embedded-cluster-upgrade-install-2", "install-b")
				return clientfake.NewClientBuilder().
					WithObjects(namespace, job1, job2).
					Build()
			},
			expectedErr: false,
		},
		{
			name:         "successfully handles no upgrade jobs found",
			installation: installation,
			setupClient: func(t *testing.T) client.Client {
				return clientfake.NewClientBuilder().
					WithObjects(namespace).
					Build()
			},
			expectedErr: false,
		},
		{
			name:         "returns error when list upgrade jobs fails",
			installation: installation,
			setupClient: func(t *testing.T) client.Client {
				return &mockClient{
					fake: clientfake.NewClientBuilder().WithObjects(namespace).Build(),
					listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						return errors.New("list failed")
					},
				}
			},
			expectedErr:    true,
			expectedErrMsg: "list upgrade jobs: list failed",
		},
		{
			name:         "returns error when delete fails for first job",
			installation: installation,
			setupClient: func(t *testing.T) client.Client {
				job1 := createUpgradeJob("embedded-cluster-upgrade-install-1", "install-a")
				job2 := createUpgradeJob("embedded-cluster-upgrade-install-2", "install-b")
				return &mockClient{
					fake: clientfake.NewClientBuilder().
						WithObjects(namespace, job1, job2).
						Build(),
					deleteFunc: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
						// Fail on first job deletion
						if obj.GetName() == "embedded-cluster-upgrade-install-1" {
							return errors.New("delete failed")
						}
						return nil
					},
				}
			},
			expectedErr:    true,
			expectedErrMsg: "delete upgrade job embedded-cluster-upgrade-install-1: delete failed",
		},
		{
			name:         "returns error when delete fails for second job",
			installation: installation,
			setupClient: func(t *testing.T) client.Client {
				job1 := createUpgradeJob("embedded-cluster-upgrade-install-1", "install-a")
				job2 := createUpgradeJob("embedded-cluster-upgrade-install-2", "install-b")
				return &mockClient{
					fake: clientfake.NewClientBuilder().
						WithObjects(namespace, job1, job2).
						Build(),
					deleteFunc: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
						// Fail on second job deletion
						if obj.GetName() == "embedded-cluster-upgrade-install-2" {
							return errors.New("delete failed")
						}
						return nil
					},
				}
			},
			expectedErr:    true,
			expectedErrMsg: "delete upgrade job embedded-cluster-upgrade-install-2: delete failed",
		},
		{
			name:         "ignores jobs without upgrade labels",
			installation: installation,
			setupClient: func(t *testing.T) client.Client {
				upgradeJob := createUpgradeJob("embedded-cluster-upgrade-install-1", "install-a")
				regularJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "regular-job",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"app.kubernetes.io/name": "regular-app",
						},
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "regular",
										Image: "regular-image",
									},
								},
								RestartPolicy: corev1.RestartPolicyNever,
							},
						},
					},
				}
				return clientfake.NewClientBuilder().
					WithObjects(namespace, upgradeJob, regularJob).
					Build()
			},
			expectedErr: false,
			validateFn: func(t *testing.T, cli client.Client) {
				// Verify that the regular job still exists (was not deleted)
				regularJob := &batchv1.Job{}
				err := cli.Get(context.Background(), client.ObjectKey{
					Name:      "regular-job",
					Namespace: "kotsadm",
				}, regularJob)
				require.NoError(t, err, "regular job should still exist")
				assert.Equal(t, "regular-job", regularJob.Name)
				assert.Equal(t, "kotsadm", regularJob.Namespace)
			},
		},
		{
			name:         "skips deleting jobs from newer installations",
			installation: installation, // install-b
			setupClient: func(t *testing.T) client.Client {
				olderJob := createUpgradeJob("embedded-cluster-upgrade-install-1", "install-a")
				sameJob := createUpgradeJob("embedded-cluster-upgrade-install-2", "install-b")
				newerJob := createUpgradeJob("embedded-cluster-upgrade-install-3", "install-c")
				return clientfake.NewClientBuilder().
					WithObjects(namespace, olderJob, sameJob, newerJob).
					Build()
			},
			expectedErr: false,
			validateFn: func(t *testing.T, cli client.Client) {
				// Verify that the newer job still exists (was not deleted)
				newerJob := &batchv1.Job{}
				err := cli.Get(context.Background(), client.ObjectKey{
					Name:      "embedded-cluster-upgrade-install-3",
					Namespace: "kotsadm",
				}, newerJob)
				require.NoError(t, err, "newer installation job should still exist")
				assert.Equal(t, "embedded-cluster-upgrade-install-3", newerJob.Name)
				assert.Equal(t, "install-c", newerJob.Annotations["embedded-cluster.replicated.com/installation-name"])

				// Verify that the older job was deleted
				olderJob := &batchv1.Job{}
				err = cli.Get(context.Background(), client.ObjectKey{
					Name:      "embedded-cluster-upgrade-install-1",
					Namespace: "kotsadm",
				}, olderJob)
				require.True(t, k8serrors.IsNotFound(err), "older installation job should have been deleted")

				// Verify that the same installation job was deleted
				sameJob := &batchv1.Job{}
				err = cli.Get(context.Background(), client.ObjectKey{
					Name:      "embedded-cluster-upgrade-install-2",
					Namespace: "kotsadm",
				}, sameJob)
				require.True(t, k8serrors.IsNotFound(err), "same installation job should have been deleted")
			},
		},
		{
			name:         "deletes jobs without installation annotation",
			installation: installation,
			setupClient: func(t *testing.T) client.Client {
				// Job without annotation should be deleted
				jobWithoutAnnotation := createUpgradeJob("embedded-cluster-upgrade-install-1", "")
				jobWithAnnotation := createUpgradeJob("embedded-cluster-upgrade-install-2", "install-a")
				return clientfake.NewClientBuilder().
					WithObjects(namespace, jobWithoutAnnotation, jobWithAnnotation).
					Build()
			},
			expectedErr: false,
			validateFn: func(t *testing.T, cli client.Client) {
				// Verify that both jobs were deleted (job without annotation gets deleted)
				job1 := &batchv1.Job{}
				err := cli.Get(context.Background(), client.ObjectKey{
					Name:      "embedded-cluster-upgrade-install-1",
					Namespace: "kotsadm",
				}, job1)
				require.True(t, k8serrors.IsNotFound(err), "job without annotation should have been deleted")

				job2 := &batchv1.Job{}
				err = cli.Get(context.Background(), client.ObjectKey{
					Name:      "embedded-cluster-upgrade-install-2",
					Namespace: "kotsadm",
				}, job2)
				require.True(t, k8serrors.IsNotFound(err), "job with older annotation should have been deleted")
			},
		},
		{
			name:         "skips active upgrade jobs",
			installation: installation,
			setupClient: func(t *testing.T) client.Client {
				// Create an active job (Active > 0)
				activeJob := createUpgradeJob("embedded-cluster-upgrade-install-active", "install-a")
				activeJob.Status.Active = 1

				// Create a completed job (Active = 0)
				completedJob := createUpgradeJob("embedded-cluster-upgrade-install-completed", "install-a")
				completedJob.Status.Active = 0

				return clientfake.NewClientBuilder().
					WithObjects(namespace, activeJob, completedJob).
					Build()
			},
			expectedErr: false,
			validateFn: func(t *testing.T, cli client.Client) {
				// Verify that the active job still exists (was not deleted)
				activeJob := &batchv1.Job{}
				err := cli.Get(context.Background(), client.ObjectKey{
					Name:      "embedded-cluster-upgrade-install-active",
					Namespace: "kotsadm",
				}, activeJob)
				require.NoError(t, err, "active job should still exist")
				assert.Equal(t, "embedded-cluster-upgrade-install-active", activeJob.Name)
				assert.Equal(t, int32(1), activeJob.Status.Active)

				// Verify that the completed job was deleted
				completedJob := &batchv1.Job{}
				err = cli.Get(context.Background(), client.ObjectKey{
					Name:      "embedded-cluster-upgrade-install-completed",
					Namespace: "kotsadm",
				}, completedJob)
				require.True(t, k8serrors.IsNotFound(err), "completed job should have been deleted")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cli := tt.setupClient(t)
			reconciler := &InstallationReconciler{
				Client: cli,
			}

			// Create a mock logger
			verbosity := 1
			if os.Getenv("DEBUG") != "" {
				verbosity = 10
			}
			logger := testr.NewWithOptions(t, testr.Options{Verbosity: verbosity})
			ctx := logr.NewContext(context.Background(), logger)

			// Execute
			err := reconciler.deleteUpgradeJobs(ctx, cli, tt.installation)

			// Verify
			if tt.expectedErr {
				require.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				require.NoError(t, err)
			}

			if tt.validateFn != nil {
				tt.validateFn(t, cli)
			}
		})
	}
}

// mockClient implements client.Client interface with customizable behavior
type mockClient struct {
	fake       client.Client
	createFunc func(context.Context, client.Object, ...client.CreateOption) error
	updateFunc func(context.Context, client.Object, ...client.UpdateOption) error
	patchFunc  func(context.Context, client.Object, client.Patch, ...client.PatchOption) error
	getFunc    func(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error
	listFunc   func(context.Context, client.ObjectList, ...client.ListOption) error
	deleteFunc func(context.Context, client.Object, ...client.DeleteOption) error
}

func (m *mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if m.getFunc != nil {
		return m.getFunc(ctx, key, obj, opts...)
	}
	return m.fake.Get(ctx, key, obj, opts...)
}

func (m *mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if m.listFunc != nil {
		return m.listFunc(ctx, list, opts...)
	}
	return m.fake.List(ctx, list, opts...)
}

func (m *mockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, obj, opts...)
	}
	return m.fake.Create(ctx, obj, opts...)
}

func (m *mockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, obj, opts...)
	}
	return m.fake.Delete(ctx, obj, opts...)
}

func (m *mockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, obj, opts...)
	}
	return m.fake.Update(ctx, obj, opts...)
}

func (m *mockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if m.patchFunc != nil {
		return m.patchFunc(ctx, obj, patch, opts...)
	}
	return m.fake.Patch(ctx, obj, patch, opts...)
}

func (m *mockClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	return m.fake.Apply(ctx, obj, opts...)
}

func (m *mockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return m.fake.DeleteAllOf(ctx, obj, opts...)
}

func (m *mockClient) Status() client.StatusWriter {
	return m.fake.Status()
}

func (m *mockClient) Scheme() *runtime.Scheme {
	return m.fake.Scheme()
}

func (m *mockClient) RESTMapper() meta.RESTMapper {
	return m.fake.RESTMapper()
}

func (m *mockClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return m.fake.GroupVersionKindFor(obj)
}

func (m *mockClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return m.fake.IsObjectNamespaced(obj)
}

func (m *mockClient) SubResource(subResource string) client.SubResourceClient {
	return m.fake.SubResource(subResource)
}
