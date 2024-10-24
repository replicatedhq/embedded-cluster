package integration

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func configMapExists(t *testing.T, kcli client.Client, name string, namespace string) bool {
	var cm corev1.ConfigMap
	err := kcli.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &cm)
	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		t.Fatalf("failed to get resource %s in namespace %s: %v", name, namespace, err)
	}
	return true
}

func secretExists(t *testing.T, kcli client.Client, name string, namespace string) bool {
	var secret corev1.Secret
	err := kcli.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		t.Fatalf("failed to get resource %s in namespace %s: %v", name, namespace, err)
	}
	return true
}
