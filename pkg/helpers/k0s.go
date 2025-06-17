package helpers

import (
	"encoding/json"
	"fmt"
	"time"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// K0sClusterConfigTo129Compat converts a k0s cluster config to a 1.29 compatible cluster config.
// This changes the chart timeout from a string to a duration. 1.30+ is backwards compatible
// and time timeout can be set as either.
func K0sClusterConfigTo129Compat(clusterConfig *k0sv1beta1.ClusterConfig) (*unstructured.Unstructured, error) {
	clusterConfig.TypeMeta.APIVersion = k0sv1beta1.ClusterConfigAPIVersion
	clusterConfig.TypeMeta.Kind = k0sv1beta1.ClusterConfigKind

	obj, err := objectToUnstructured(clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("convert to unstructured: %w", err)
	}
	unst := obj.UnstructuredContent()

	// check the entire spec path before attempting to access "charts"
	if unst["spec"] == nil {
		return obj, nil
	}
	if _, ok := unst["spec"].(map[string]interface{}); !ok {
		return obj, nil
	}
	if _, ok := unst["spec"].(map[string]interface{})["extensions"]; !ok {
		return obj, nil
	}
	if _, ok := unst["spec"].(map[string]interface{})["extensions"].(map[string]interface{}); !ok {
		return obj, nil
	}
	if _, ok := unst["spec"].(map[string]interface{})["extensions"].(map[string]interface{})["helm"]; !ok {
		return obj, nil
	}
	if _, ok := unst["spec"].(map[string]interface{})["extensions"].(map[string]interface{})["helm"].(map[string]interface{}); !ok {
		return obj, nil
	}
	if _, ok := unst["spec"].(map[string]interface{})["extensions"].(map[string]interface{})["helm"].(map[string]interface{})["charts"]; !ok {
		return obj, nil
	}

	charts, ok := unst["spec"].(map[string]interface{})["extensions"].(map[string]interface{})["helm"].(map[string]interface{})["charts"].([]interface{})
	if !ok {
		return obj, nil
	}
	for _, chart := range charts {
		chartMap := chart.(map[string]interface{})
		d, err := timeoutStringToDuration(chartMap["timeout"].(string))
		if err != nil {
			return nil, err
		}
		chartMap["timeout"] = d
	}
	return obj, nil
}

func timeoutStringToDuration(str string) (time.Duration, error) {
	if str == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(str)
	if err != nil {
		return 0, fmt.Errorf("parse duration: %w", err)
	}
	return d, nil
}

func objectToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	objMap := map[string]interface{}{}
	err = json.Unmarshal(data, &objMap)
	if err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	unstructuredObj := &unstructured.Unstructured{}
	unstructuredObj.Object = objMap
	return unstructuredObj, nil
}

func NetworkSpecFromK0sConfig(k0sCfg *k0sv1beta1.ClusterConfig) ecv1beta1.NetworkSpec {
	network := ecv1beta1.NetworkSpec{}

	if k0sCfg.Spec != nil && k0sCfg.Spec.Network != nil {
		network.PodCIDR = k0sCfg.Spec.Network.PodCIDR
		network.ServiceCIDR = k0sCfg.Spec.Network.ServiceCIDR
	}

	if k0sCfg.Spec.API != nil {
		if val, ok := k0sCfg.Spec.API.ExtraArgs["service-node-port-range"]; ok {
			network.NodePortRange = val
		}
	}

	return network
}
