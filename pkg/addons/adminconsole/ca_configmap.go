package adminconsole

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	PrivateCASConfigMapName = "kotsadm-private-cas"
)

func EnsureCAConfigmap(ctx context.Context, logf types.LogFunc, kcli client.Client, mcli metadata.Interface, caPath string) error {
	if caPath == "" {
		return nil
	}

	checksum, err := calculateFileChecksum(caPath)
	if err != nil {
		return fmt.Errorf("calculate checksum: %w", err)
	}

	existingMeta, err := mcli.Resource(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}).Namespace(constants.KotsadmNamespace).Get(ctx, PrivateCASConfigMapName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("get configmap metadata: %w", err)
	}

	if existingMeta != nil {
		existingChecksum := existingMeta.Annotations["replicated.com/cas-checksum"]
		if existingChecksum == checksum {
			return nil
		}
	}

	new, err := newCAConfigMap(caPath, checksum)
	if err != nil {
		return fmt.Errorf("create map: %w", err)
	} else if new == nil {
		return nil
	}

	err = kcli.Create(ctx, new)
	if err == nil {
		logf("Created %s ConfigMap", PrivateCASConfigMapName)
		return nil
	} else if !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create configmap: %w", err)
	}

	marshalledData, err := json.Marshal(new.Data)
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}

	rawPatch := fmt.Sprintf(`{"metadata":{"annotations":{"replicated.com/cas-checksum":"%s"}},"data":%s}`, checksum, marshalledData)
	patch := client.RawPatch(apitypes.MergePatchType, []byte(rawPatch))
	err = kcli.Patch(ctx, new, patch)
	if err != nil {
		return fmt.Errorf("patch configmap: %w", err)
	}
	logf("Updated %s ConfigMap", PrivateCASConfigMapName)

	return nil
}

func newCAConfigMap(caPath string, checksum string) (*corev1.ConfigMap, error) {
	if caPath == "" {
		return nil, nil
	}

	casMap, err := casToMap([]string{caPath})
	if err != nil {
		return nil, fmt.Errorf("create map: %w", err)
	}

	return casConfigMap(casMap, checksum), nil
}

func calculateFileChecksum(path string) (string, error) {
	hasher := md5.New()

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(hasher, file)
	if err != nil {
		return "", fmt.Errorf("copy file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func casConfigMap(cas map[string]string, checksum string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      PrivateCASConfigMapName,
			Namespace: constants.KotsadmNamespace,
			Labels: map[string]string{
				"kots.io/kotsadm":                        "true",
				"replicated.com/disaster-recovery":       "infra",
				"replicated.com/disaster-recovery-chart": "admin-console",
			},
			Annotations: map[string]string{
				"replicated.com/cas-checksum": checksum,
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
