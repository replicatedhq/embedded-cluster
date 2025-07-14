package artifacts

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestEnsureArtifactsJobForNodes(t *testing.T) {
	removeJobFinalizers := func(ctx context.Context, t *testing.T, cli client.Client, in *clusterv1beta1.Installation) {
		for {
			timer := time.NewTimer(1 * time.Second)
			select {
			case <-timer.C:
			case <-ctx.Done():
				return
			}

			list := &batchv1.JobList{}
			err := cli.List(context.Background(), list, client.InNamespace(ecNamespace))
			if err != nil {
				require.NoError(t, err)
			}
			for _, item := range list.Items {
				if item.GetFinalizers() != nil {
					// there is no job controller in envtest so the finalizer will not be
					// removed and the job will not be deleted
					item.SetFinalizers(nil)
					err := cli.Update(context.Background(), &item)
					require.NoError(t, err)
				}
			}
		}
	}

	type args struct {
		in                       *clusterv1beta1.Installation
		localArtifactMirrorImage string
		licenseID                string
		appSlug                  string
		channelID                string
		appVersion               string
	}
	tests := []struct {
		name            string
		initRuntimeObjs []client.Object
		modifyRuntime   func(ctx context.Context, t *testing.T, cli client.Client, in *clusterv1beta1.Installation)
		args            args
		wantErr         bool
		assertRuntime   func(t *testing.T, cli client.Client, in *clusterv1beta1.Installation)
	}{
		{
			name: "create artifacts job",
			initRuntimeObjs: []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: ecNamespace,
					},
				},
			},
			args: args{
				in: &clusterv1beta1.Installation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-installation",
					},
					Spec: clusterv1beta1.InstallationSpec{
						AirGap: true,
						Artifacts: &clusterv1beta1.ArtifactsLocation{
							Images:                  "images",
							HelmCharts:              "helm-charts",
							EmbeddedClusterBinary:   "embedded-cluster-binary",
							EmbeddedClusterMetadata: "embedded-cluster-metadata",
						},
					},
				},
				localArtifactMirrorImage: "local-artifact-mirror",
				licenseID:                "abcd1234",
				appSlug:                  "app-slug",
				channelID:                "channel-id",
				appVersion:               "1.0.0",
			},
			wantErr: false,
			assertRuntime: func(t *testing.T, cli client.Client, in *clusterv1beta1.Installation) {
				artifactsHash, err := hashForAirgapConfig(in)
				require.NoError(t, err)

				job := &batchv1.Job{}

				err = cli.Get(context.Background(), client.ObjectKey{Namespace: ecNamespace, Name: copyArtifactsJobPrefix + "node1"}, job)
				require.NoError(t, err)

				assert.Equal(t, "test-installation", job.Annotations[InstallationNameAnnotation])
				assert.Equal(t, artifactsHash, job.Annotations[ArtifactsConfigHashAnnotation])
				assert.Equal(t, "local-artifact-mirror", job.Spec.Template.Spec.Containers[0].Image)

				err = cli.Get(context.Background(), client.ObjectKey{Namespace: ecNamespace, Name: copyArtifactsJobPrefix + "node2"}, job)
				require.NoError(t, err)

				assert.Equal(t, "test-installation", job.Annotations[InstallationNameAnnotation])
				assert.Equal(t, artifactsHash, job.Annotations[ArtifactsConfigHashAnnotation])
				assert.Equal(t, "local-artifact-mirror", job.Spec.Template.Spec.Containers[0].Image)
			},
		},
		{
			name: "replace existing artifacts job",
			initRuntimeObjs: []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: ecNamespace,
					},
				},
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ecNamespace,
						Name:      copyArtifactsJobPrefix + "node1",
						Annotations: map[string]string{
							InstallationNameAnnotation:    "test-installation",
							ArtifactsConfigHashAnnotation: "old-hash",
						},
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyNever,
								Containers: []corev1.Container{
									{
										Name:  "copy-artifacts",
										Image: "old-image",
									},
								},
							},
						},
					},
				},
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ecNamespace,
						Name:      copyArtifactsJobPrefix + "node2",
						Annotations: map[string]string{
							InstallationNameAnnotation:    "old-installation",
							ArtifactsConfigHashAnnotation: "old-hash",
						},
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyNever,
								Containers: []corev1.Container{
									{
										Name:  "copy-artifacts",
										Image: "old-image",
									},
								},
							},
						},
					},
				},
			},
			args: args{
				in: &clusterv1beta1.Installation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-installation",
					},
					Spec: clusterv1beta1.InstallationSpec{
						AirGap: true,
						Artifacts: &clusterv1beta1.ArtifactsLocation{
							Images:                  "images",
							HelmCharts:              "helm-charts",
							EmbeddedClusterBinary:   "embedded-cluster-binary",
							EmbeddedClusterMetadata: "embedded-cluster-metadata",
						},
					},
				},
				localArtifactMirrorImage: "local-artifact-mirror",
			},
			modifyRuntime: removeJobFinalizers,
			wantErr:       false,
			assertRuntime: func(t *testing.T, cli client.Client, in *clusterv1beta1.Installation) {
				artifactsHash, err := hashForAirgapConfig(in)
				require.NoError(t, err)

				job := &batchv1.Job{}

				err = cli.Get(context.Background(), client.ObjectKey{Namespace: ecNamespace, Name: copyArtifactsJobPrefix + "node1"}, job)
				require.NoError(t, err)

				assert.Equal(t, "test-installation", job.Annotations[InstallationNameAnnotation])
				assert.Equal(t, artifactsHash, job.Annotations[ArtifactsConfigHashAnnotation])
				assert.Equal(t, "local-artifact-mirror", job.Spec.Template.Spec.Containers[0].Image)

				err = cli.Get(context.Background(), client.ObjectKey{Namespace: ecNamespace, Name: copyArtifactsJobPrefix + "node2"}, job)
				require.NoError(t, err)

				assert.Equal(t, "test-installation", job.Annotations[InstallationNameAnnotation])
				assert.Equal(t, artifactsHash, job.Annotations[ArtifactsConfigHashAnnotation])
				assert.Equal(t, "local-artifact-mirror", job.Spec.Template.Spec.Containers[0].Image)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			verbosity := 1
			if os.Getenv("DEBUG") != "" {
				verbosity = 10
			}
			log := testr.NewWithOptions(t, testr.Options{Verbosity: verbosity})
			ctx = logr.NewContext(ctx, log)

			testEnv := &envtest.Environment{}
			cfg, err := testEnv.Start()
			require.NoError(t, err)
			t.Cleanup(func() { _ = testEnv.Stop() })

			cli, err := client.New(cfg, client.Options{Scheme: kubeutils.Scheme})
			require.NoError(t, err)

			for _, obj := range tt.initRuntimeObjs {
				err := cli.Create(ctx, obj)
				require.NoError(t, err)
			}

			wg := sync.WaitGroup{}
			if tt.modifyRuntime != nil {
				wg.Add(1)
				go func() {
					tt.modifyRuntime(ctx, t, cli, tt.args.in)
					wg.Done()
				}()
			}

			rc := runtimeconfig.New(tt.args.in.Spec.RuntimeConfig)

			if err := EnsureArtifactsJobForNodes(
				ctx, cli, rc, tt.args.in,
				tt.args.localArtifactMirrorImage,
				tt.args.licenseID,
				tt.args.appSlug,
				tt.args.channelID,
				tt.args.appVersion,
			); (err != nil) != tt.wantErr {
				t.Errorf("EnsureArtifactsJobForNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			tt.assertRuntime(t, cli, tt.args.in)

			cancel()

			wg.Wait()
		})
	}
}

func TestListArtifactsJobForNodes(t *testing.T) {
	in := &clusterv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-installation",
		},
		Spec: clusterv1beta1.InstallationSpec{
			AirGap: true,
			Artifacts: &clusterv1beta1.ArtifactsLocation{
				Images:                  "images",
				HelmCharts:              "helm-charts",
				EmbeddedClusterBinary:   "embedded-cluster-binary",
				EmbeddedClusterMetadata: "embedded-cluster-metadata",
			},
		},
	}

	type args struct {
		in *clusterv1beta1.Installation
	}
	tests := []struct {
		name            string
		initRuntimeObjs []client.Object
		args            args
		wantErr         bool
		assertWant      func(t *testing.T, in *clusterv1beta1.Installation, want map[string]*batchv1.Job)
	}{
		{
			name: "list artifacts job",
			initRuntimeObjs: []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node3",
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: ecNamespace,
					},
				},
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ecNamespace,
						Name:      copyArtifactsJobPrefix + "node1",
						Annotations: map[string]string{
							InstallationNameAnnotation: "test-installation",
							ArtifactsConfigHashAnnotation: func() string {
								artifactsHash, err := hashForAirgapConfig(in)
								require.NoError(t, err)
								return artifactsHash
							}(),
						},
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyNever,
								Containers: []corev1.Container{
									{
										Name:  "copy-artifacts",
										Image: "image",
									},
								},
							},
						},
					},
				},
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ecNamespace,
						Name:      copyArtifactsJobPrefix + "node2",
						Annotations: map[string]string{
							InstallationNameAnnotation:    "test-installation",
							ArtifactsConfigHashAnnotation: "old-hash",
						},
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyNever,
								Containers: []corev1.Container{
									{
										Name:  "copy-artifacts",
										Image: "image",
									},
								},
							},
						},
					},
				},
			},
			args: args{
				in: in,
			},
			wantErr: false,
			assertWant: func(t *testing.T, in *clusterv1beta1.Installation, want map[string]*batchv1.Job) {
				artifactsHash, err := hashForAirgapConfig(in)
				require.NoError(t, err)

				assert.Len(t, want, 3)

				job := want["node1"]
				assert.Equal(t, "test-installation", job.Annotations[InstallationNameAnnotation])
				assert.Equal(t, artifactsHash, job.Annotations[ArtifactsConfigHashAnnotation])

				// old hash
				job = want["node2"]
				assert.Nil(t, job)

				// missing
				job = want["node3"]
				assert.Nil(t, job)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verbosity := 1
			if os.Getenv("DEBUG") != "" {
				verbosity = 10
			}
			log := testr.NewWithOptions(t, testr.Options{Verbosity: verbosity})
			ctx := logr.NewContext(context.Background(), log)

			testEnv := &envtest.Environment{}
			cfg, err := testEnv.Start()
			require.NoError(t, err)
			t.Cleanup(func() { _ = testEnv.Stop() })

			cli, err := client.New(cfg, client.Options{Scheme: kubeutils.Scheme})
			require.NoError(t, err)

			for _, obj := range tt.initRuntimeObjs {
				err := cli.Create(ctx, obj)
				require.NoError(t, err)
			}

			got, err := ListArtifactsJobForNodes(ctx, cli, tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListArtifactsJobForNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.assertWant != nil {
				tt.assertWant(t, tt.args.in, got)
			}
		})
	}
}

func TestGetArtifactJobForNode_HostCABundle(t *testing.T) {
	// Test with HostCABundlePath set
	t.Run("with HostCABundlePath set", func(t *testing.T) {
		verbosity := 1
		if os.Getenv("DEBUG") != "" {
			verbosity = 10
		}
		log := testr.NewWithOptions(t, testr.Options{Verbosity: verbosity})
		ctx := logr.NewContext(context.Background(), log)

		scheme := runtime.NewScheme()
		require.NoError(t, clusterv1beta1.AddToScheme(scheme))
		require.NoError(t, batchv1.AddToScheme(scheme))
		require.NoError(t, corev1.AddToScheme(scheme))

		// CA path used for testing
		testCAPath := "/etc/ssl/certs/ca-certificates.crt"

		// Create a minimal installation CR with RuntimeConfig.HostCABundlePath set
		installation := &clusterv1beta1.Installation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-installation",
			},
			Spec: clusterv1beta1.InstallationSpec{
				AirGap: true,
				Artifacts: &clusterv1beta1.ArtifactsLocation{
					Images:                  "images",
					HelmCharts:              "helm-charts",
					EmbeddedClusterBinary:   "embedded-cluster-binary",
					EmbeddedClusterMetadata: "embedded-cluster-metadata",
				},
				RuntimeConfig: &clusterv1beta1.RuntimeConfigSpec{
					HostCABundlePath: testCAPath,
				},
			},
		}

		// Create a fake client
		cli := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(installation).
			Build()

		// Create a test node
		node := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
		}

		rc := runtimeconfig.New(installation.Spec.RuntimeConfig)

		// Call the function under test
		job, err := getArtifactJobForNode(
			ctx, cli, rc, installation, node,
			"local-artifact-mirror:latest",
			"app-slug",
			"channel-id",
			"1.0.0",
		)
		require.NoError(t, err)

		// Verify that the host CA bundle volume exists
		var hostCABundleVolumeFound bool
		for _, volume := range job.Spec.Template.Spec.Volumes {
			if volume.Name == "host-ca-bundle" {
				hostCABundleVolumeFound = true
				// Verify the volume properties
				require.NotNil(t, volume.HostPath, "Host CA bundle volume should be a hostPath volume")
				assert.Equal(t, testCAPath, volume.HostPath.Path, "Host CA bundle path should match RuntimeConfig.HostCABundlePath")
				assert.Equal(t, corev1.HostPathFileOrCreate, *volume.HostPath.Type, "Host CA bundle type should be FileOrCreate")
				break
			}
		}
		assert.True(t, hostCABundleVolumeFound, "Host CA bundle volume should exist")

		// Verify that the volume mount exists
		var hostCABundleMountFound bool
		for _, mount := range job.Spec.Template.Spec.Containers[0].VolumeMounts {
			if mount.Name == "host-ca-bundle" {
				hostCABundleMountFound = true
				// Verify the mount properties
				assert.Equal(t, "/certs/ca-certificates.crt", mount.MountPath, "Host CA bundle mount path should be correct")
				break
			}
		}
		assert.True(t, hostCABundleMountFound, "Host CA bundle mount should exist")

		// Verify that the SSL_CERT_DIR environment variable exists
		var sslCertDirEnvFound bool
		for _, env := range job.Spec.Template.Spec.Containers[0].Env {
			if env.Name == "SSL_CERT_DIR" {
				sslCertDirEnvFound = true
				// Verify the env var value
				assert.Equal(t, "/certs", env.Value, "SSL_CERT_DIR value should be correct")
				break
			}
		}
		assert.True(t, sslCertDirEnvFound, "SSL_CERT_DIR environment variable should exist")
	})

	// Test without HostCABundlePath set
	t.Run("without HostCABundlePath set", func(t *testing.T) {
		verbosity := 1
		if os.Getenv("DEBUG") != "" {
			verbosity = 10
		}
		log := testr.NewWithOptions(t, testr.Options{Verbosity: verbosity})
		ctx := logr.NewContext(context.Background(), log)

		scheme := runtime.NewScheme()
		require.NoError(t, clusterv1beta1.AddToScheme(scheme))
		require.NoError(t, batchv1.AddToScheme(scheme))
		require.NoError(t, corev1.AddToScheme(scheme))

		// Create a minimal installation CR without RuntimeConfig.HostCABundlePath
		installation := &clusterv1beta1.Installation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-installation",
			},
			Spec: clusterv1beta1.InstallationSpec{
				AirGap: true,
				Artifacts: &clusterv1beta1.ArtifactsLocation{
					Images:                  "images",
					HelmCharts:              "helm-charts",
					EmbeddedClusterBinary:   "embedded-cluster-binary",
					EmbeddedClusterMetadata: "embedded-cluster-metadata",
				},
				// No RuntimeConfig or empty RuntimeConfig
			},
		}

		// Create a fake client
		cli := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(installation).
			Build()

		// Create a test node
		node := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
		}

		rc := runtimeconfig.New(installation.Spec.RuntimeConfig)

		// Call the function under test
		job, err := getArtifactJobForNode(
			ctx, cli, rc, installation, node,
			"local-artifact-mirror:latest",
			"app-slug",
			"channel-id",
			"1.0.0",
		)
		require.NoError(t, err)

		// Verify that the host CA bundle volume does NOT exist
		var hostCABundleVolumeFound bool
		for _, volume := range job.Spec.Template.Spec.Volumes {
			if volume.Name == "host-ca-bundle" {
				hostCABundleVolumeFound = true
				break
			}
		}
		assert.False(t, hostCABundleVolumeFound, "Host CA bundle volume should not exist when HostCABundlePath is not set")

		// Verify that the volume mount does NOT exist
		var hostCABundleMountFound bool
		for _, mount := range job.Spec.Template.Spec.Containers[0].VolumeMounts {
			if mount.Name == "host-ca-bundle" {
				hostCABundleMountFound = true
				break
			}
		}
		assert.False(t, hostCABundleMountFound, "Host CA bundle mount should not exist when HostCABundlePath is not set")

		// Verify that the SSL_CERT_DIR environment variable does NOT exist
		var sslCertDirEnvFound bool
		for _, env := range job.Spec.Template.Spec.Containers[0].Env {
			if env.Name == "SSL_CERT_DIR" {
				sslCertDirEnvFound = true
				break
			}
		}
		assert.False(t, sslCertDirEnvFound, "SSL_CERT_DIR environment variable should not exist when HostCABundlePath is not set")
	})
}
