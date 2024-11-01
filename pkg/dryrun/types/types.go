package types

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

type DryRun struct {
	Flags             map[string]interface{}                 `json:"flags"`
	Commands          []Command                              `json:"commands"`
	Metrics           []Metric                               `json:"metrics"`
	HostPreflightSpec *troubleshootv1beta2.HostPreflightSpec `json:"hostPreflightSpec"`

	// These fields are set on marshal
	OSEnv      map[string]string `json:"osEnv"`
	K8sObjects []string          `json:"k8sObjects"`

	// These fields are used as mocks
	kcli client.Client `json:"-"`
}

type Metric struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Payload string `json:"payload"`
}

type Command struct {
	Cmd string            `json:"cmd"`
	Env map[string]string `json:"env,omitempty"`
}

func (d *DryRun) MarshalJSON() ([]byte, error) {
	k8sObjects, err := d.K8sObjectsFromClient()
	if err != nil {
		return nil, fmt.Errorf("get k8s objects: %w", err)
	}
	alias := *d
	alias.OSEnv = getOSEnv()
	alias.K8sObjects = k8sObjects
	return json.Marshal(alias)
}

func (d *DryRun) K8sObjectsFromClient() ([]string, error) {
	kcli, err := d.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("get kube client: %w", err)
	}

	ctx := context.Background()
	result := []string{}

	addToResult := func(o runtime.Object) error {
		data, err := yaml.Marshal(o)
		if err != nil {
			return fmt.Errorf("marshal object: %w", err)
		}
		result = append(result, string(data))
		return nil
	}

	// Services
	var services corev1.ServiceList
	if err := kcli.List(ctx, &services); err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	for _, svc := range services.Items {
		if err := addToResult(&svc); err != nil {
			return nil, fmt.Errorf("add to result: %w", err)
		}
	}

	// Deployments
	var deployments appsv1.DeploymentList
	if err := kcli.List(ctx, &deployments); err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	for _, dpl := range deployments.Items {
		if err := addToResult(&dpl); err != nil {
			return nil, fmt.Errorf("add to result: %w", err)
		}
	}

	// StatefulSets
	var statefulSets appsv1.StatefulSetList
	if err := kcli.List(ctx, &statefulSets); err != nil {
		return nil, fmt.Errorf("list statefulsets: %w", err)
	}
	for _, sts := range statefulSets.Items {
		if err := addToResult(&sts); err != nil {
			return nil, fmt.Errorf("add to result: %w", err)
		}
	}

	// DaemonSets
	var daemonSets appsv1.DaemonSetList
	if err := kcli.List(ctx, &daemonSets); err != nil {
		return nil, fmt.Errorf("list daemonsets: %w", err)
	}
	for _, ds := range daemonSets.Items {
		if err := addToResult(&ds); err != nil {
			return nil, fmt.Errorf("add to result: %w", err)
		}
	}

	// Nodes
	var nodes corev1.NodeList
	if err := kcli.List(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	for _, node := range nodes.Items {
		if err := addToResult(&node); err != nil {
			return nil, fmt.Errorf("add to result: %w", err)
		}
	}

	// ConfigMaps
	var configMaps corev1.ConfigMapList
	if err := kcli.List(ctx, &configMaps); err != nil {
		return nil, fmt.Errorf("list configmaps: %w", err)
	}
	for _, cm := range configMaps.Items {
		if err := addToResult(&cm); err != nil {
			return nil, fmt.Errorf("add to result: %w", err)
		}
	}

	// Secrets
	var secrets corev1.SecretList
	if err := kcli.List(ctx, &secrets); err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	for _, secret := range secrets.Items {
		if err := addToResult(&secret); err != nil {
			return nil, fmt.Errorf("add to result: %w", err)
		}
	}

	// Roles
	var roles rbacv1.RoleList
	if err := kcli.List(ctx, &roles); err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	for _, role := range roles.Items {
		if err := addToResult(&role); err != nil {
			return nil, fmt.Errorf("add to result: %w", err)
		}
	}

	// RoleBindings
	var roleBindings rbacv1.RoleBindingList
	if err := kcli.List(ctx, &roleBindings); err != nil {
		return nil, fmt.Errorf("list rolebindings: %w", err)
	}
	for _, rb := range roleBindings.Items {
		if err := addToResult(&rb); err != nil {
			return nil, fmt.Errorf("add to result: %w", err)
		}
	}

	// Installation CRs
	var installations ecv1beta1.InstallationList
	if err := kcli.List(ctx, &installations); err != nil {
		return nil, fmt.Errorf("list installations: %w", err)
	}
	for _, install := range installations.Items {
		if err := addToResult(&install); err != nil {
			return nil, fmt.Errorf("add to result: %w", err)
		}
	}

	return result, nil
}

func (d *DryRun) KubeClient() (client.Client, error) {
	if d.kcli == nil {
		scheme := runtime.NewScheme()
		if err := k8scheme.AddToScheme(scheme); err != nil {
			return nil, fmt.Errorf("add k8s scheme: %w", err)
		}
		if err := ecv1beta1.AddToScheme(scheme); err != nil {
			return nil, fmt.Errorf("add ec v1beta1 scheme: %w", err)
		}
		clientObjs := []client.Object{}
		for _, o := range d.K8sObjects {
			var m map[string]interface{}
			if err := yaml.Unmarshal([]byte(o), &m); err != nil {
				return nil, fmt.Errorf("unmarshal: %w", err)
			}
			clientObjs = append(clientObjs, &unstructured.Unstructured{Object: m})
		}
		d.kcli = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(clientObjs...).
			Build()
	}
	return d.kcli, nil
}

func getOSEnv() map[string]string {
	osEnv := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			osEnv[parts[0]] = parts[1]
		}
	}
	return osEnv
}
