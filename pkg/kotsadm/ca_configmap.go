package kotsadm

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kotsadmNamespace        = "kotsadm"
	privateCASConfigMapName = "kotsadm-private-cas"
)

// LogFunc can be used as an argument to Run to log messages.
type LogFunc func(string, ...any)

func EnsureCAConfigmap(ctx context.Context, logf LogFunc, kcli client.Client, caPath string, retries int) error {
	if caPath == "" {
		return nil
	}

	var err error
	for range retries + 1 {
		err = ensureCAConfigmap(ctx, logf, kcli, caPath)
		if err == nil {
			return nil
		}
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return err
}

func ensureCAConfigmap(ctx context.Context, logf LogFunc, kcli client.Client, caPath string) error {
	if caPath == "" {
		return nil
	}

	casMap, err := casToMap([]string{caPath})
	if err != nil {
		return fmt.Errorf("create map: %w", err)
	}
	ca, ok := casMap["ca_0.crt"]
	if !ok {
		return fmt.Errorf("ca_0.crt not found in map")
	}

	existing := casConfigMap(casMap)
	err = kcli.Get(ctx, client.ObjectKeyFromObject(existing), existing)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			new := casConfigMap(casMap)
			err := kcli.Create(ctx, new)
			if err != nil {
				return fmt.Errorf("create configmap: %w", err)
			}
			logf("Created %s ConfigMap", privateCASConfigMapName)
			return nil
		}
		return fmt.Errorf("get configmap: %w", err)
	}

	existingCA, ok := existing.Data["ca_0.crt"]
	if ok && existingCA == ca {
		return nil
	}

	existing.Data = casMap

	err = kcli.Update(ctx, existing)
	if err != nil {
		return fmt.Errorf("update configmap: %w", err)
	}
	logf("Updated %s ConfigMap", privateCASConfigMapName)

	return nil
}

func casConfigMap(cas map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      privateCASConfigMapName,
			Namespace: kotsadmNamespace,
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
