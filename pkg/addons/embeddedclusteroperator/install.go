package embeddedclusteroperator

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (e *EmbeddedClusterOperator) Install(ctx context.Context, kcli client.Client, hcli helm.Client, overrides []string, writer *spinner.MessageWriter) error {
	err := installEnsureCAConfigmap(ctx, kcli, e.PrivateCAs)
	if err != nil {
		return errors.Wrap(err, "ensure CA configmap")
	}

	values, err := e.GenerateHelmValues(ctx, kcli, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = hcli.Install(ctx, helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    e.ChartLocation(),
		ChartVersion: e.ChartVersion(),
		Values:       values,
		Namespace:    namespace,
		Labels:       getBackupLabels(),
	})
	if err != nil {
		return errors.Wrap(err, "helm install")
	}

	return nil
}

func installEnsureCAConfigmap(ctx context.Context, kcli client.Client, privateCAs []string) error {
	cas, err := privateCAsToMap(privateCAs)
	if err != nil {
		return errors.Wrap(err, "create private cas map")
	}
	return ensureCAConfigmap(ctx, kcli, cas)
}

func ensureCAConfigmap(ctx context.Context, cli client.Client, cas map[string]string) error {
	kotsCAConfigmap := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "private-cas",
			Namespace: namespace,
			Labels:    getBackupLabels(),
		},
		Data: cas,
	}

	err := cli.Create(ctx, &kotsCAConfigmap)
	if client.IgnoreAlreadyExists(err) != nil {
		return errors.Wrap(err, "create kotsadm-private-cas configmap")
	}

	return nil
}

func privateCAsToMap(privateCAs []string) (map[string]string, error) {
	cas := map[string]string{}
	for i, path := range privateCAs {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, errors.Wrapf(err, "read private CA file %s", path)
		}
		name := fmt.Sprintf("ca_%d.crt", i)
		cas[name] = string(data)
	}
	return cas, nil
}
