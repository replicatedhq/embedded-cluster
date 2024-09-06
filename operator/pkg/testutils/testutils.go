package testutils

import (
	"testing"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

func Scheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	require.NoError(t, err)
	err = clusterv1beta1.SchemeBuilder.AddToScheme(scheme)
	require.NoError(t, err)
	return scheme
}

func Installation(options ...func(*clusterv1beta1.Installation)) *clusterv1beta1.Installation {
	in := &clusterv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1beta1.GroupVersion.String(),
			Kind:       "Installation",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "embedded-cluster-kinds",
			Generation: int64(2),
		},
		Spec: clusterv1beta1.InstallationSpec{
			BinaryName: "binary-name",
			ClusterID:  "cluster-id",
			Config: &clusterv1beta1.ConfigSpec{
				Version: "version",
			},
		},
	}
	for _, option := range options {
		option(in)
	}
	return in
}

func OwnerReference() metav1.OwnerReference {
	in := Installation()
	return metav1.OwnerReference{
		APIVersion:         clusterv1beta1.GroupVersion.String(),
		Kind:               "Installation",
		Name:               in.GetName(),
		UID:                in.GetUID(),
		BlockOwnerDeletion: ptr.To(true),
		Controller:         ptr.To(true),
	}
}
