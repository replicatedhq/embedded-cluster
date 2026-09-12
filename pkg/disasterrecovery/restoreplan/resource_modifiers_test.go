package restoreplan

import (
	"encoding/json"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/stretchr/testify/require"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type modifierConfig struct {
	Version string         `json:"version"`
	Rules   []modifierRule `json:"resourceModifierRules"`
}

type modifierRule struct {
	Conditions struct {
		GroupResource     string   `json:"groupResource"`
		ResourceNameRegex string   `json:"resourceNameRegex"`
		Namespaces        []string `json:"namespaces"`
	} `json:"conditions"`
	Patches []struct {
		Operation string `json:"operation"`
		Path      string `json:"path"`
		Value     any    `json:"value"`
	} `json:"patches"`
	MergePatches []struct {
		PatchData string `json:"patchData"`
	} `json:"mergePatches"`
}

func TestResourceModifiersApplyProductionRestorePlan(t *testing.T) {
	backup := &velerov1.Backup{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		airgapAnnotation: "true", highAvailabilityAnnotation: "true",
		registryAnnotation: "10.96.0.10:5000", seaweedFSAnnotation: "10.96.0.11",
	}}}
	rendered, err := ResourceModifiers(backup)
	require.NoError(t, err)
	require.NotContains(t, rendered, "__REGISTRY_SERVICE_IP__")
	require.NotContains(t, rendered, "__SEAWEEDFS_S3_SERVICE_IP__")

	var config modifierConfig
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &config))
	require.Equal(t, "v1", config.Version)
	require.Len(t, config.Rules, 4)

	rqlite := applyJSONPatches(t, []byte(`{"spec":{"replicas":3,"template":{"spec":{"containers":[{"args":["rqlited","-disco-mode=dns","-bootstrap-expect=3"]}]}}}}`), config.Rules[0])
	require.JSONEq(t, `{"spec":{"replicas":1,"template":{"spec":{"containers":[{"args":["rqlited","-disco-mode=dns","-bootstrap-expect=1"]}]}}}}`, string(rqlite))

	pvc, err := jsonpatch.MergePatch([]byte(`{"metadata":{"annotations":{"volume.kubernetes.io/selected-node":"node-1","keep":"yes"}}}`), []byte(config.Rules[1].MergePatches[0].PatchData))
	require.NoError(t, err)
	require.JSONEq(t, `{"metadata":{"annotations":{"keep":"yes"}}}`, string(pvc))

	registry := applyJSONPatches(t, []byte(`{"spec":{}}`), config.Rules[2])
	require.JSONEq(t, `{"spec":{"clusterIP":"10.96.0.10"}}`, string(registry))
	seaweed := applyJSONPatches(t, []byte(`{"spec":{}}`), config.Rules[3])
	require.JSONEq(t, `{"spec":{"clusterIP":"10.96.0.11"}}`, string(seaweed))

	require.Equal(t, "statefulsets.apps", config.Rules[0].Conditions.GroupResource)
	require.Equal(t, "^kotsadm-rqlite$", config.Rules[0].Conditions.ResourceNameRegex)
	require.Equal(t, []string{"kotsadm"}, config.Rules[0].Conditions.Namespaces)
	require.Equal(t, "persistentvolumeclaims", config.Rules[1].Conditions.GroupResource)
	require.Equal(t, "services", config.Rules[2].Conditions.GroupResource)
	require.Equal(t, []string{"registry"}, config.Rules[2].Conditions.Namespaces)
	require.Equal(t, []string{"seaweedfs"}, config.Rules[3].Conditions.Namespaces)
}

func TestResourceModifiersRequireBackupMetadata(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		wantError   string
	}{
		{name: "missing airgap", annotations: map[string]string{}, wantError: "airgap annotation not found"},
		{name: "airgap missing registry", annotations: map[string]string{airgapAnnotation: "true"}, wantError: "registry service IP"},
		{name: "airgap missing HA", annotations: map[string]string{airgapAnnotation: "true", registryAnnotation: "10.0.0.1:5000"}, wantError: "high availability annotation"},
		{name: "HA missing SeaweedFS", annotations: map[string]string{airgapAnnotation: "true", registryAnnotation: "10.0.0.1:5000", highAvailabilityAnnotation: "true"}, wantError: "SeaweedFS S3 service IP"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResourceModifiers(&velerov1.Backup{ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations}})
			require.ErrorContains(t, err, tt.wantError)
		})
	}
}

func applyJSONPatches(t *testing.T, document []byte, rule modifierRule) []byte {
	t.Helper()
	operations := make([]map[string]any, 0, len(rule.Patches))
	for _, patch := range rule.Patches {
		operations = append(operations, map[string]any{"op": patch.Operation, "path": patch.Path, "value": patch.Value})
	}
	data, err := json.Marshal(operations)
	require.NoError(t, err)
	patch, err := jsonpatch.DecodePatch(data)
	require.NoError(t, err)
	result, err := patch.Apply(document)
	require.NoError(t, err)
	return result
}
