package adminconsole

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	privateCASConfigMapName = "kotsadm-private-cas"
)

func EnsureCAConfigmap(ctx context.Context, logf types.LogFunc, kcli client.Client, caPath string) error {
	new, err := newCAConfigMap(caPath)
	if err != nil {
		return fmt.Errorf("create map: %w", err)
	} else if new == nil {
		return nil
	}

	var existing corev1.ConfigMap
	err = kcli.Get(ctx, client.ObjectKeyFromObject(new), &existing)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			err := kcli.Create(ctx, new)
			if err != nil {
				return fmt.Errorf("create configmap: %w", err)
			}
			logf("Created %s ConfigMap", privateCASConfigMapName)
			return nil
		}
		return fmt.Errorf("get configmap: %w", err)
	}

	newCA := new.Data["ca_0.crt"]
	existingCA := existing.Data["ca_0.crt"]
	if existingCA == newCA {
		return nil
	}

	existing.Data = new.Data

	err = kcli.Update(ctx, &existing)
	if err != nil {
		return fmt.Errorf("update configmap: %w", err)
	}
	logf("Updated %s ConfigMap", privateCASConfigMapName)

	return nil
}

func newCAConfigMap(caPath string) (*corev1.ConfigMap, error) {
	if caPath == "" {
		return nil, nil
	}

	casMap, err := casToMap([]string{caPath})
	if err != nil {
		return nil, fmt.Errorf("create map: %w", err)
	}

	return casConfigMap(casMap), nil
}

func casConfigMap(cas map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      privateCASConfigMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"kots.io/kotsadm":                        "true",
				"replicated.com/disaster-recovery":       "infra",
				"replicated.com/disaster-recovery-chart": "admin-console",
			},
		},
		Data: cas,
	}
}

func casToMap(cas []string) (map[string]string, error) {
	casMap := map[string]string{}
	for i, path := range cas {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read ca file %s: %w", path, err)
		}
		name := fmt.Sprintf("ca_%d.crt", i)
		casMap[name] = string(data)
	}
	return casMap, nil
}
