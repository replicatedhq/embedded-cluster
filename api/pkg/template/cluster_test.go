package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEngine_NodeCount(t *testing.T) {
	tests := []struct {
		name           string
		nodes          []corev1.Node
		template       string
		expectedResult string
	}{
		{
			name: "NodeCount with 3 nodes",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-3"}},
			},
			template:       "{{repl NodeCount }}",
			expectedResult: "3",
		},
		{
			name:           "NodeCount with 0 nodes",
			nodes:          []corev1.Node{},
			template:       "{{repl NodeCount }}",
			expectedResult: "0",
		},
		{
			name: "NodeCount in string interpolation",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
			},
			template:       "replicas: {{repl NodeCount }}",
			expectedResult: "replicas: 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake client with the nodes
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			objects := make([]client.Object, len(tt.nodes))
			for i := range tt.nodes {
				objects[i] = &tt.nodes[i]
			}

			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			// Create engine
			engine := NewEngine(nil, WithMode(ModeGeneric))

			// Parse and execute template
			err := engine.Parse(tt.template)
			require.NoError(t, err)

			result, err := engine.Execute(nil, WithKubeClient(kubeClient))
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestEngine_NodeCountIntegrated(t *testing.T) {
	tests := []struct {
		name           string
		nodes          []corev1.Node
		template       string
		expectedResult string
	}{
		{
			name: "NodeCount with arithmetic - subtraction",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-3"}},
			},
			template:       "{{repl Sub NodeCount 1 }}",
			expectedResult: "2",
		},
		{
			name: "NodeCount with arithmetic - addition",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
			},
			template:       "{{repl Add NodeCount 1 }}",
			expectedResult: "3",
		},
		{
			name: "NodeCount with arithmetic - multiplication",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
			},
			template:       "{{repl Mult NodeCount 2 }}",
			expectedResult: "4",
		},
		{
			name: "NodeCount in conditional - less than",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
			},
			template:       "{{repl if (lt NodeCount 3) }}small{{repl else }}large{{repl end }}",
			expectedResult: "small",
		},
		{
			name: "NodeCount in conditional - greater than or equal",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-3"}},
			},
			template:       "{{repl if (ge NodeCount 3) }}large{{repl else }}small{{repl end }}",
			expectedResult: "large",
		},
		{
			name: "NodeCount with ternary operator",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
			},
			template:       "{{repl lt NodeCount 3 | ternary \"single-node\" \"multi-node\" }}",
			expectedResult: "single-node",
		},
		{
			name: "NodeCount with multiple operations",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-3"}},
			},
			template:       "nodes: {{repl NodeCount }}, replicas: {{repl Sub NodeCount 1 }}",
			expectedResult: "nodes: 3, replicas: 2",
		},
		{
			name: "NodeCount equality check - true",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
			},
			template:       "{{repl if (eq NodeCount 1) }}single{{repl else }}multiple{{repl end }}",
			expectedResult: "single",
		},
		{
			name: "NodeCount equality check - false",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
			},
			template:       "{{repl if (eq NodeCount 1) }}single{{repl else }}multiple{{repl end }}",
			expectedResult: "multiple",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake client with the nodes
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			objects := make([]client.Object, len(tt.nodes))
			for i := range tt.nodes {
				objects[i] = &tt.nodes[i]
			}

			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			// Create engine
			engine := NewEngine(nil, WithMode(ModeGeneric))

			// Parse and execute template
			err := engine.Parse(tt.template)
			require.NoError(t, err)

			result, err := engine.Execute(nil, WithKubeClient(kubeClient))
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestEngine_NodeCountWithoutKubeClient(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		expectError bool
	}{
		{
			name:        "NodeCount without kube client fails during execution",
			template:    "{{repl NodeCount }}",
			expectError: true,
		},
		{
			name:        "NodeCount in conditional without kube client fails",
			template:    "{{repl if (lt NodeCount 3) }}small{{repl else }}large{{repl end }}",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create engine without kube client
			engine := NewEngine(nil, WithMode(ModeGeneric))

			// Parse template
			err := engine.Parse(tt.template)
			require.NoError(t, err)

			// Execute should fail because nodeCount requires a kube client
			_, err = engine.Execute(nil)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
