package util

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func EnsureInstallation(t *testing.T, kcli client.Client, spec ecv1beta1.InstallationSpec) {
	err := embeddedclusteroperator.EnsureInstallationCRD(t.Context(), kcli)
	if err != nil {
		t.Fatalf("failed to create installation CRD: %v", err)
	}

	in := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
	}

	err = kcli.Get(t.Context(), client.ObjectKeyFromObject(in), in)
	if client.IgnoreNotFound(err) != nil {
		t.Fatalf("failed to get installation: %v", err)
	} else if err == nil {
		in.Spec = spec
		err = kcli.Update(t.Context(), in)
		if err != nil {
			t.Fatalf("failed to update installation: %v", err)
		}
		return
	}

	in.Spec = spec
	err = kcli.Create(t.Context(), in)
	if err != nil {
		t.Fatalf("failed to create installation: %v", err)
	}
}
