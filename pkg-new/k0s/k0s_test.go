package k0s

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	k8syaml "sigs.k8s.io/yaml"
)

//go:embed testdata/*
var testData embed.FS

func TestPatchK0sConfig(t *testing.T) {
	type test struct {
		Name     string
		Original string `yaml:"original"`
		Override string `yaml:"override"`
		Expected string `yaml:"expected"`
	}
	for tname, tt := range parseTestsYAML[test](t, "patch-k0s-config-") {
		t.Run(tname, func(t *testing.T) {
			req := require.New(t)

			originalFile, err := os.CreateTemp("", "k0s-original-*.yaml")
			req.NoError(err, "unable to create temp file")
			defer func() {
				originalFile.Close()
				os.Remove(originalFile.Name())
			}()
			err = os.WriteFile(originalFile.Name(), []byte(tt.Original), 0644)
			req.NoError(err, "unable to write original config")

			var patch string
			if tt.Override != "" {
				var overrides embeddedclusterv1beta1.Config
				err = k8syaml.Unmarshal([]byte(tt.Override), &overrides)
				req.NoError(err, "unable to unmarshal override")
				patch = overrides.Spec.UnsupportedOverrides.K0s
			}

			err = PatchK0sConfig(originalFile.Name(), patch)
			req.NoError(err, "unable to patch config")

			data, err := os.ReadFile(originalFile.Name())
			req.NoError(err, "unable to read patched config")

			var original k0sv1beta1.ClusterConfig
			err = k8syaml.Unmarshal(data, &original)
			req.NoError(err, "unable to decode original file")

			var expected k0sv1beta1.ClusterConfig
			err = k8syaml.Unmarshal([]byte(tt.Expected), &expected)
			req.NoError(err, "unable to unmarshal expected file")

			assert.Equal(t, expected, original)
		})
	}
}

func parseTestsYAML[T any](t *testing.T, prefix string) map[string]T {
	entries, err := testData.ReadDir("testdata")
	require.NoError(t, err)
	tests := make(map[string]T, 0)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}

		fpath := filepath.Join("testdata", entry.Name())
		data, err := testData.ReadFile(fpath)
		require.NoError(t, err)

		var onetest T
		err = yaml.Unmarshal(data, &onetest)
		require.NoError(t, err)

		tests[fpath] = onetest
	}
	return tests
}

func TestWaitForAutopilotPlan_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))

	plan := &apv1b2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
		},
		Status: apv1b2.PlanStatus{
			State: core.PlanCompleted,
		},
	}

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(plan).
		Build()

	result, err := (&K0s{}).WaitForAutopilotPlan(t.Context(), cli, logger)
	require.NoError(t, err)
	assert.Equal(t, "autopilot", result.Name)
}

func TestWaitForAutopilotPlan_RetriesOnTransientErrors(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))

	// Plan that starts completed
	plan := &apv1b2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
		},
		Status: apv1b2.PlanStatus{
			State: core.PlanCompleted,
		},
	}

	// Mock client that fails first 3 times, then succeeds
	var callCount atomic.Int32
	cli := &mockClientWithRetries{
		Client:       fake.NewClientBuilder().WithScheme(scheme).WithObjects(plan).Build(),
		failCount:    3,
		currentCount: &callCount,
	}

	result, err := (&K0s{}).waitForAutopilotPlanWithBackoff(t.Context(), cli, logger, wait.Backoff{
		Duration: 1 * time.Millisecond,
		Steps:    10,
	})
	require.NoError(t, err)
	assert.Equal(t, "autopilot", result.Name)
	assert.Equal(t, int32(4), callCount.Load(), "Should have retried 3 times before succeeding")
}

func TestWaitForAutopilotPlan_ContextCanceled(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	_, err := (&K0s{}).WaitForAutopilotPlan(ctx, cli, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestWaitForAutopilotPlan_WaitsForCompletion(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))

	// Plan that starts in progress, then completes after some time
	plan := &apv1b2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
		},
		Spec: apv1b2.PlanSpec{
			ID: "test-plan",
		},
		Status: apv1b2.PlanStatus{
			State: core.PlanSchedulable,
		},
	}

	cli := &mockClientWithStateChange{
		Client:     fake.NewClientBuilder().WithScheme(scheme).WithObjects(plan).Build(),
		plan:       plan,
		callsUntil: 3, // Will complete after 3 calls
	}

	result, err := (&K0s{}).waitForAutopilotPlanWithBackoff(t.Context(), cli, logger, wait.Backoff{
		Duration: 1 * time.Millisecond,
		Steps:    10,
	})
	require.NoError(t, err)
	assert.Equal(t, "autopilot", result.Name)
	assert.Equal(t, core.PlanCompleted, result.Status.State)
}

func TestWaitForAutopilotPlan_LongRunningUpgrade(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))

	// Simulate a long-running upgrade that takes more than 5 attempts
	// This represents a real k0s infrastructure upgrade that takes several minutes
	plan := &apv1b2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
		},
		Spec: apv1b2.PlanSpec{
			ID: "long-running-upgrade",
		},
		Status: apv1b2.PlanStatus{
			State: core.PlanSchedulable,
		},
	}

	cli := &mockClientWithStateChange{
		Client:     fake.NewClientBuilder().WithScheme(scheme).WithObjects(plan).Build(),
		plan:       plan,
		callsUntil: 10, // Will complete after 10 calls - exceeds buggy 5-attempt limit
	}

	result, err := (&K0s{}).waitForAutopilotPlanWithBackoff(t.Context(), cli, logger, wait.Backoff{
		Duration: 1 * time.Millisecond,
		Steps:    20,
	})
	require.NoError(t, err, "Should not timeout for long-running upgrades")
	assert.Equal(t, "autopilot", result.Name)
	assert.Equal(t, core.PlanCompleted, result.Status.State)
}

// Mock client that fails N times before succeeding
type mockClientWithRetries struct {
	client.Client
	failCount    int
	currentCount *atomic.Int32
}

func (m *mockClientWithRetries) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	count := m.currentCount.Add(1)
	if count <= int32(m.failCount) {
		return fmt.Errorf("connection refused")
	}
	return m.Client.Get(ctx, key, obj, opts...)
}

// Mock client that changes plan state after N calls
type mockClientWithStateChange struct {
	client.Client
	plan       *apv1b2.Plan
	callCount  int
	callsUntil int
}

func (m *mockClientWithStateChange) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	m.callCount++
	err := m.Client.Get(ctx, key, obj, opts...)
	if err != nil {
		return err
	}

	// After N calls, mark the plan as completed
	if m.callCount >= m.callsUntil {
		if plan, ok := obj.(*apv1b2.Plan); ok {
			plan.Status.State = core.PlanCompleted
		}
	}

	return nil
}

func TestWaitForClusterNodesMatchVersion(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	tests := []struct {
		name          string
		nodes         *corev1.NodeList
		targetVersion string
		mockClient    func(*corev1.NodeList) client.Client
		backoff       *wait.Backoff
		expectError   bool
		errorContains string
		validate      func(t *testing.T, cli client.Client)
	}{
		{
			name: "all nodes already match version",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.30.0+k0s",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node2"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.30.0+k0s",
							},
						},
					},
				},
			},
			targetVersion: "v1.30.0+k0s",
			mockClient: func(nodes *corev1.NodeList) client.Client {
				return fake.NewClientBuilder().WithScheme(scheme).WithLists(nodes).Build()
			},
			expectError: false,
		},
		{
			name: "nodes update after retries",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.29.0+k0s",
							},
						},
					},
				},
			},
			targetVersion: "v1.30.0+k0s",
			mockClient: func(nodes *corev1.NodeList) client.Client {
				return &mockClientWithNodeVersionUpdate{
					Client:         fake.NewClientBuilder().WithScheme(scheme).WithLists(nodes).Build(),
					callsUntil:     3,
					targetVersion:  "v1.30.0+k0s",
					initialVersion: "v1.29.0+k0s",
				}
			},
			backoff: &wait.Backoff{
				Duration: 1 * time.Millisecond,
				Steps:    10,
			},
			expectError: false,
			validate: func(t *testing.T, cli client.Client) {
				if mock, ok := cli.(*mockClientWithNodeVersionUpdate); ok {
					assert.Equal(t, 3, mock.callCount, "Should have retried until nodes reported correct version")
				}
			},
		},
		{
			name: "multi-node staggered updates",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.29.0+k0s",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node2"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.29.0+k0s",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node3"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.29.0+k0s",
							},
						},
					},
				},
			},
			targetVersion: "v1.30.0+k0s",
			mockClient: func(nodes *corev1.NodeList) client.Client {
				return &mockClientWithStaggeredNodeUpdates{
					Client:        fake.NewClientBuilder().WithScheme(scheme).WithLists(nodes).Build(),
					targetVersion: "v1.30.0+k0s",
				}
			},
			backoff: &wait.Backoff{
				Duration: 1 * time.Millisecond,
				Steps:    20,
			},
			expectError: false,
			validate: func(t *testing.T, cli client.Client) {
				if mock, ok := cli.(*mockClientWithStaggeredNodeUpdates); ok {
					assert.GreaterOrEqual(t, mock.callCount, 3, "Should have waited for all nodes to update")
				}
			},
		},
		{
			name: "timeout when nodes never match",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.29.0+k0s",
							},
						},
					},
				},
			},
			targetVersion: "v1.30.0+k0s",
			mockClient: func(nodes *corev1.NodeList) client.Client {
				return fake.NewClientBuilder().WithScheme(scheme).WithLists(nodes).Build()
			},
			backoff: &wait.Backoff{
				Duration: 100 * time.Millisecond,
				Steps:    3,
			},
			expectError:   true,
			errorContains: "cluster nodes did not match version v1.30.0+k0s after upgrade",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := tt.mockClient(tt.nodes)
			var err error
			if tt.backoff != nil {
				err = (&K0s{}).waitForClusterNodesMatchVersionWithBackoff(t.Context(), cli, tt.targetVersion, logger, *tt.backoff)
			} else {
				err = (&K0s{}).WaitForClusterNodesMatchVersion(t.Context(), cli, tt.targetVersion, logger)
			}

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, cli)
				}
			}
		})
	}
}

// Mock client that updates node versions after N calls
type mockClientWithNodeVersionUpdate struct {
	client.Client
	callCount      int
	callsUntil     int
	targetVersion  string
	initialVersion string
}

func (m *mockClientWithNodeVersionUpdate) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	m.callCount++
	err := m.Client.List(ctx, list, opts...)
	if err != nil {
		return err
	}

	if m.callCount >= m.callsUntil {
		if nodeList, ok := list.(*corev1.NodeList); ok {
			for i := range nodeList.Items {
				nodeList.Items[i].Status.NodeInfo.KubeletVersion = m.targetVersion
			}
		}
	}

	return nil
}

// Mock client that updates nodes one at a time to simulate staggered upgrades
type mockClientWithStaggeredNodeUpdates struct {
	client.Client
	callCount     int
	targetVersion string
}

func (m *mockClientWithStaggeredNodeUpdates) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	m.callCount++
	err := m.Client.List(ctx, list, opts...)
	if err != nil {
		return err
	}

	if nodeList, ok := list.(*corev1.NodeList); ok {
		for i := range nodeList.Items {
			if m.callCount > i {
				nodeList.Items[i].Status.NodeInfo.KubeletVersion = m.targetVersion
			}
		}
	}

	return nil
}

func Test_clusterNodesMatchVersion(t *testing.T) {
	tests := []struct {
		name    string
		want    bool
		version string
		objects []runtime.Object
	}{
		{
			name:    "no nodes",
			want:    true,
			version: "irrelevant",
			objects: []runtime.Object{},
		},
		{
			name:    "one node, matches version",
			want:    true,
			version: "v1.29.9+k0s",
			objects: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							KubeletVersion: "v1.29.9+k0s",
						},
					},
				},
			},
		},
		{
			name:    "one node, doesn't match version",
			want:    false,
			version: "v1.29.9+k0s",
			objects: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							KubeletVersion: "v1.29.8+k0s",
						},
					},
				},
			},
		},
		{
			name:    "two nodes, one matches version",
			want:    false,
			version: "v1.30.5+k0s",
			objects: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							KubeletVersion: "v1.29.9+k0s",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							KubeletVersion: "v1.30.5+k0s",
						},
					},
				},
			},
		},
		{
			name:    "two nodes, both match version",
			want:    true,
			version: "v1.30.5+k0s",
			objects: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							KubeletVersion: "v1.30.5+k0s",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							KubeletVersion: "v1.30.5+k0s",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			cli := fake.NewFakeClient(tt.objects...)

			got, err := (&K0s{}).ClusterNodesMatchVersion(t.Context(), cli, tt.version)
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}
