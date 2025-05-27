package embeddedclusteroperator

import (
	"bytes"
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
	err := e.installEnsureCAConfigmap(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "ensure CA configmap")
	}

	values, err := e.GenerateHelmValues(ctx, kcli, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	opts := helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    e.ChartLocation(),
		ChartVersion: e.ChartVersion(),
		Values:       values,
		Namespace:    namespace,
		Labels:       getBackupLabels(),
	}

	if e.DryRun {
		manifests, err := hcli.Render(ctx, opts)
		if err != nil {
			return errors.Wrap(err, "dry run values")
		}
		e.dryRunManifests = append(e.dryRunManifests, manifests...)
	} else {
		_, err = hcli.Install(ctx, opts)
		if err != nil {
			return errors.Wrap(err, "helm install")
		}
	}

	return nil
}

func (e *EmbeddedClusterOperator) installEnsureCAConfigmap(ctx context.Context, kcli client.Client) error {
	cas, err := privateCAsToMap(e.PrivateCAs)
	if err != nil {
		return errors.Wrap(err, "create private cas map")
	}
	return e.ensureCAConfigmap(ctx, kcli, cas)
}

func (e *EmbeddedClusterOperator) ensureCAConfigmap(ctx context.Context, cli client.Client, cas map[string]string) error {
	obj := &corev1.ConfigMap{
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

	if e.DryRun {
		b := bytes.NewBuffer(nil)
		if err := serializer.Encode(obj, b); err != nil {
			return errors.Wrap(err, "serialize")
		}
		e.dryRunManifests = append(e.dryRunManifests, b.Bytes())
		return nil
	}

	err := cli.Create(ctx, obj)
	if client.IgnoreAlreadyExists(err) != nil {
		return errors.Wrap(err, "create private-cas configmap")
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
