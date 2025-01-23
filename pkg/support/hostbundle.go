package support

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	sb "github.com/replicatedhq/troubleshoot/pkg/supportbundle"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kubectl/pkg/scheme"
)

var (
	//go:embed static/host-support-bundle-remote.yaml
	_hostSupportBundleRemote []byte
)

func GetRemoteHostSupportBundleSpec() []byte {
	return _hostSupportBundleRemote
}

func CreateHostSupportBundle() error {
	specFile := GetRemoteHostSupportBundleSpec()

	var b bytes.Buffer
	s := serializer.NewYAMLSerializer(serializer.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	hostSupportBundle, err := sb.ParseSupportBundleFromDoc(specFile)
	if err != nil {
		return fmt.Errorf("unable to unmarshal support bundle spec: %w", err)
	}

	if err := s.Encode(hostSupportBundle, &b); err != nil {
		return fmt.Errorf("unable to encode support bundle spec: %w", err)
	}

	renderedSpec := b.Bytes()

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "embedded-cluster-host-support-bundle",
			Namespace: "kotsadm",
			Labels: map[string]string{
				"troubleshoot.sh/kind":                   "support-bundle",
				"kots.io/kotsadm":                        "true",
				"replicated.com/disaster-recovery":       "infra",
				"replicated.com/disaster-recovery-chart": "admin-console",
			},
		},
		Data: map[string]string{
			"support-bundle-spec": string(renderedSpec),
		},
	}

	ctx := context.Background()
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	err = kcli.Create(ctx, configMap)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("unable to create config map: %w", err)
	}

	if errors.IsAlreadyExists(err) {
		if err := kcli.Update(ctx, configMap); err != nil {
			return fmt.Errorf("unable to update config map: %w", err)
		}
	}

	return nil
}
