package config

import (
	"strings"
	"testing"

	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/k0sproject/k0s/pkg/constant"
)

func TestListK0sImages(t *testing.T) {
	original := airgap.GetImageURIs(RenderK0sConfig().Spec, true)
	if len(original) == 0 {
		t.Errorf("airgap.GetImageURIs() = %v, want not empty", original)
	}
	var foundKubeRouter, foundCNINode, foundKonnectivity bool
	for _, image := range original {
		if strings.Contains(image, "kube-router") {
			foundKubeRouter = true
		}
		if strings.Contains(image, "cni-node") {
			foundCNINode = true
		}
		if strings.Contains(image, "apiserver-network-proxy-agent") {
			foundKonnectivity = true
		}
	}
	if !foundKubeRouter {
		t.Errorf("ListK0sImages() = %v, want to contain kube-router", original)
	}
	if !foundCNINode {
		t.Errorf("ListK0sImages() = %v, want to contain kube-router", original)
	}
	if !foundKonnectivity {
		t.Errorf("ListK0sImages() = %v, want to contain apiserver-network-proxy-agent", original)
	}

	filtered := ListK0sImages(RenderK0sConfig())
	if len(filtered) == 0 {
		t.Errorf("ListK0sImages() = %v, want not empty", filtered)
	}
	for _, image := range filtered {
		if strings.Contains(image, "kube-router") {
			t.Errorf("ListK0sImages() = %v, want not to contain kube-router", filtered)
		}
		if strings.Contains(image, "cni-node") {
			t.Errorf("ListK0sImages() = %v, want not to contain kube-router", filtered)
		}
		if strings.Contains(image, "apiserver-network-proxy-agent") {
			t.Errorf("ListK0sImages() = %v, want not to contain apiserver-network-proxy-agent", filtered)
		}
		if strings.Contains(image, constant.KubePauseContainerImage) {
			t.Errorf("ListK0sImages() = %v, want the ec pause image", filtered)
		}
	}
}
