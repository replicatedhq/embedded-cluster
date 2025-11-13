package template

import (
	"context"
	"fmt"
	"testing"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestEngine_NodeCount(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	t.Run("returns 0 when kubeClient is nil", func(t *testing.T) {
		engine := NewEngine(config)

		err := engine.Parse("{{repl NodeCount }}")
		require.NoError(t, err)
		result, err := engine.Execute(nil)
		require.NoError(t, err)
		assert.Equal(t, "0", result)
	})

	t.Run("returns correct count with multiple nodes", func(t *testing.T) {
		// Set up scheme for fake client
		sch := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(sch))
		require.NoError(t, scheme.AddToScheme(sch))

		// Create fake nodes
		node1 := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
		}
		node2 := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
			},
		}
		node3 := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-3",
			},
		}

		// Create fake client with nodes
		fakeClient := clientfake.NewClientBuilder().
			WithScheme(sch).
			WithObjects(node1, node2, node3).
			Build()

		engine := NewEngine(config, WithKubeClient(fakeClient))

		err := engine.Parse("{{repl NodeCount }}")
		require.NoError(t, err)
		result, err := engine.Execute(nil)
		require.NoError(t, err)
		assert.Equal(t, "3", result)
	})

	t.Run("can be used in conditional expressions", func(t *testing.T) {
		// Set up scheme for fake client
		sch := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(sch))
		require.NoError(t, scheme.AddToScheme(sch))

		// Create fake nodes
		node1 := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
		}
		node2 := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
			},
		}

		// Create fake client with nodes
		fakeClient := clientfake.NewClientBuilder().
			WithScheme(sch).
			WithObjects(node1, node2).
			Build()

		engine := NewEngine(config, WithKubeClient(fakeClient))

		// Test conditional expression
		err := engine.Parse("{{repl if gt (NodeCount) 1 }}HA{{repl else }}Standalone{{repl end }}")
		require.NoError(t, err)
		result, err := engine.Execute(nil)
		require.NoError(t, err)
		assert.Equal(t, "HA", result)
	})

	t.Run("returns 0 when kubeClient List returns error", func(t *testing.T) {
		// Set up scheme for fake client
		sch := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(sch))
		require.NoError(t, scheme.AddToScheme(sch))

		// Create fake client with interceptor that returns error on List
		fakeClient := clientfake.NewClientBuilder().
			WithScheme(sch).
			WithInterceptorFuncs(interceptor.Funcs{
				List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					return fmt.Errorf("simulated list error")
				},
			}).
			Build()

		engine := NewEngine(config, WithKubeClient(fakeClient))

		err := engine.Parse("{{repl NodeCount }}")
		require.NoError(t, err)
		result, err := engine.Execute(nil)
		require.NoError(t, err)
		assert.Equal(t, "0", result)
	})
}
