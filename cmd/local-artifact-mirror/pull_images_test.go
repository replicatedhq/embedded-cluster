package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPullImagesCmd(t *testing.T) {
	// Create temporary directory for test
	dataDir := t.TempDir()
	t.Setenv("TMPDIR", dataDir) // hack as the cli sets TMPDIR, this will reset it after the test

	rc := runtimeconfig.New(nil)
	rc.SetDataDir(dataDir)

	// Create a fake client with test Installation
	scheme := runtime.NewScheme()
	err := ecv1beta1.AddToScheme(scheme)
	require.NoError(t, err)

	// Create a test Installation
	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "airgap-installation",
		},
		Spec: ecv1beta1.InstallationSpec{
			AirGap: true,
			Artifacts: &ecv1beta1.ArtifactsLocation{
				Images: "registry.example.com/images:latest",
			},
		},
	}

	// Create a non-airgap installation for error case
	nonAirgapInstallation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-airgap-installation",
		},
		Spec: ecv1beta1.InstallationSpec{
			AirGap: false,
		},
	}

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation, nonAirgapInstallation).
		Build()

	testCases := []struct {
		name          string
		setupEnv      func(t *testing.T)
		args          []string
		mock          *mockPuller
		expectedError string
	}{
		{
			name: "successful pull with env var",
			args: []string{installation.Name},
			setupEnv: func(t *testing.T) {
				t.Setenv("LOCAL_ARTIFACT_MIRROR_DATA_DIR", dataDir)
			},
			mock: func() *mockPuller {
				m := &mockPuller{}
				// Create a test artifact file
				artifactDir := t.TempDir()
				m.On("PullArtifact", mock.Anything, mock.Anything, "registry.example.com/images:latest").
					Once().
					Run(func(args mock.Arguments) {
						artifactFile := filepath.Join(artifactDir, ImagesSrcArtifactName)
						fmt.Println("artifactFile", artifactFile)
						err = helpers.WriteFile(artifactFile, []byte("test artifact content"), 0644)
						require.NoError(t, err)
					}).
					Return(artifactDir, nil)
				return m
			}(),
			expectedError: "",
		},
		{
			name: "successful pull with Flag",
			args: []string{installation.Name, "--data-dir", dataDir},
			mock: func() *mockPuller {
				m := &mockPuller{}
				// Create a test artifact file
				artifactDir := t.TempDir()
				require.NoError(t, err)
				m.On("PullArtifact", mock.Anything, mock.Anything, "registry.example.com/images:latest").
					Once().
					Run(func(args mock.Arguments) {
						artifactFile := filepath.Join(artifactDir, ImagesSrcArtifactName)
						fmt.Println("artifactFile", artifactFile)
						err = helpers.WriteFile(artifactFile, []byte("test artifact content"), 0644)
						require.NoError(t, err)
					}).
					Return(artifactDir, nil)
				return m
			}(),
			expectedError: "",
		},
		{
			name: "pull artifact failure",
			args: []string{installation.Name},
			setupEnv: func(t *testing.T) {
				t.Setenv("LOCAL_ARTIFACT_MIRROR_DATA_DIR", dataDir)
			},
			mock: func() *mockPuller {
				m := &mockPuller{}
				m.On("PullArtifact", mock.Anything, mock.Anything, "registry.example.com/images:latest").
					Once().
					Return("", errors.New("failed to pull artifact"))
				return m
			}(),
			expectedError: "unable to fetch artifact: failed to pull artifact",
		},
		{
			name: "move file failure - source file doesn't exist",
			args: []string{installation.Name},
			setupEnv: func(t *testing.T) {
				t.Setenv("LOCAL_ARTIFACT_MIRROR_DATA_DIR", dataDir)
			},
			mock: func() *mockPuller {
				m := &mockPuller{}
				// Create another dir for failed pulls
				emptyDir := t.TempDir()
				m.On("PullArtifact", mock.Anything, mock.Anything, "registry.example.com/images:latest").
					Once().
					Return(emptyDir, nil)
				return m
			}(),
			expectedError: "unable to move images bundle",
		},
		{
			name: "non-airgap installation",
			args: []string{nonAirgapInstallation.Name},
			setupEnv: func(t *testing.T) {
				t.Setenv("LOCAL_ARTIFACT_MIRROR_DATA_DIR", dataDir)
			},
			mock: func() *mockPuller {
				m := &mockPuller{}
				return m
			}(),
			expectedError: "pulling images is not supported for online installations",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup environment for test
			if tc.setupEnv != nil {
				tc.setupEnv(t)
			}

			// Create the command
			cli := &CLI{
				RC:   rc,
				Name: "local-artifact-mirror",
				V:    viper.New(),
				KCLIGetter: func() (client.Client, error) {
					return fakeClient, nil
				},
				PullArtifact: tc.mock.PullArtifact,
			}
			root := RootCmd(cli)

			// Execute command
			_, _, err := testExecuteCommandC(t.Context(), root, append([]string{"pull", "images"}, tc.args...)...)

			tc.mock.AssertExpectations(t)

			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)

				// Check that the destination file exists
				expectedDst := filepath.Join(dataDir, "images", ImagesDstArtifactName)
				_, err := os.Stat(expectedDst)
				assert.NoError(t, err, "Expected destination file to exist")

				// Verify file content
				content, err := os.ReadFile(expectedDst)
				assert.NoError(t, err)
				assert.Equal(t, "test artifact content", string(content))
			}
		})
	}
}

type mockPuller struct {
	mock.Mock
}

func (m *mockPuller) PullArtifact(ctx context.Context, kcli client.Client, from string) (string, error) {
	args := m.Called(ctx, kcli, from)
	return args.String(0), args.Error(1)
}
