// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/replicatedhq/helmvm/pkg/customization"
	"github.com/replicatedhq/helmvm/pkg/defaults"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
	"github.com/replicatedhq/helmvm/pkg/prompts"
)

const (
	releaseName = "admin-console"
)

// Overwritten by -ldflags in Makefile
var (
	ChartURL  = "https://url"
	ChartName = "name"
	Version   = "v0.0.0"
)

var helmValues = map[string]interface{}{
	"password":      "password",
	"minimalRBAC":   false,
	"isHelmManaged": false,
	"service": map[string]interface{}{
		"type":     "NodePort",
		"nodePort": 30000,
	},
}

// AdminConsole manages the admin console helm chart installation.
type AdminConsole struct {
	customization customization.AdminConsole
	namespace     string
	useprompt     bool
	config        v1beta1.ClusterConfig
}

func (a *AdminConsole) askPassword() (string, error) {

	defaultPass := "password"

	if !a.useprompt {
		fmt.Println("Admin Console password set to: password")
		return defaultPass, nil
	}

	maxTries := 3
	for i := 0; i < maxTries; i++ {
		promptA := prompts.New().Password("Enter a new Admin Console password:")
		promptB := prompts.New().Password("Confirm password:")

		if promptA == promptB {
			return promptA, nil
		}
		fmt.Println("Passwords don't match, please try again.")
	}

	return "", fmt.Errorf("Unable to set Admin Console password after %d tries", maxTries)

}

// Version returns the embedded admin console version.
func (a *AdminConsole) Version() (map[string]string, error) {
	return map[string]string{"AdminConsole": "v" + Version}, nil
}

// HostPreflights returns the host preflight objects found inside the adminconsole
// or as part of the embedded kots release (customization).
func (a *AdminConsole) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return a.customization.HostPreflights()
}

// addLicenseToHelmValues adds the embedded license to the helm values.
func (a *AdminConsole) addLicenseToHelmValues() error {
	license, err := a.customization.License()
	if err != nil {
		return fmt.Errorf("unable to get license: %w", err)
	}
	if license == nil {
		return nil
	}
	raw, err := yaml.Marshal(license)
	if err != nil {
		return fmt.Errorf("unable to marshal license: %w", err)
	}
	helmValues["automation"] = map[string]interface{}{
		"license": map[string]interface{}{
			"slug": license.Spec.AppSlug,
			"data": string(raw),
		},
	}
	return nil
}

// GetPasswordFromConfig returns the adminconsole password from the provided chart config.
func getPasswordFromConfig(chart v1beta1.Chart) (string, error) {

	values := dig.Mapping{}

	if chart.Values == "" {
		return "", fmt.Errorf("unable to find adminconsole chart values in cluster config")
	}

	err := yaml.Unmarshal([]byte(chart.Values), &values)
	if err != nil {
		return "", fmt.Errorf("unable to unmarshal values: %w", err)
	}

	if password, ok := values["password"].(string); ok {
		return password, nil
	}

	return "", fmt.Errorf("unable to find password in cluster config")

}

// GetCurrentConfig returns the current adminconsole chart config from the cluster config.
func (a *AdminConsole) GetCurrentConfig() (v1beta1.Chart, error) {

	nilChart := v1beta1.Chart{}

	if a.config.Spec == nil {
		return nilChart, fmt.Errorf("unable to find spec in cluster config")
	}
	spec := a.config.Spec

	if spec.Extensions == nil {
		return nilChart, fmt.Errorf("unable to find extensions in cluster config")
	}
	extensions := spec.Extensions

	if extensions.Helm == nil {
		return nilChart, fmt.Errorf("unable to find helm extensions in cluster config")
	}
	chartList := a.config.Spec.Extensions.Helm.Charts

	for _, chart := range chartList {
		if chart.Name == "adminconsole" {
			return chart, nil
		}
	}

	return nilChart, fmt.Errorf("unable to find adminconsole chart in cluster config")

}

// GenerateHelmConfig generates the helm config for the adminconsole
// and writes the charts to the disk.
func (a *AdminConsole) GenerateHelmConfig() ([]v1beta1.Chart, []v1beta1.Repository, error) {
	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: fmt.Sprintf("%s/%s", ChartURL, ChartName),
		Version:   Version,
		Values:    "",
		TargetNS:  a.namespace,
	}

	if err := a.addLicenseToHelmValues(); err != nil {
		return nil, nil, fmt.Errorf("unable to add license to helm values: %w", err)
	}

	currentConfig, err := a.GetCurrentConfig()
	if err == nil {
		currentPassword, err := getPasswordFromConfig(currentConfig)
		if err != nil {
			pass, err := a.askPassword()
			if err != nil {
				return nil, nil, fmt.Errorf("unable to ask for password: %w", err)
			}
			helmValues["password"] = pass
		} else if currentPassword != "" {
			helmValues["password"] = currentPassword
		}
	} else {
		pass, err := a.askPassword()
		if err != nil {
			return nil, nil, fmt.Errorf("unable to ask for password: %w", err)
		}
		helmValues["password"] = pass
	}

	cust, err := customization.AdminConsole{}.ExtractCustomization()
	if err == nil {
		if cust != nil && cust.Application != nil {
			helmValues["kotsApplication"] = string(cust.Application)
		} else {
			helmValues["kotsApplication"] = "default value"
		}
	} else {
		helmValues["kotsApplication"] = "error value"
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	return []v1beta1.Chart{chartConfig}, nil, nil
}

// New creates a new AdminConsole object.
func New(ns string, useprompt bool, config v1beta1.ClusterConfig) (*AdminConsole, error) {
	return &AdminConsole{
		namespace:     ns,
		useprompt:     useprompt,
		customization: customization.AdminConsole{},
		config:        config,
	}, nil
}

// WaitFor waits for the admin console to be ready.
func WaitFor(configPath string) error {
	displayName := "Admin Console"
	namespace := "helmvm"

	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	progressBar := pb.Start()
	defer func() {
		if err != nil {
			return
		}
		progressBar.Closef("%s is ready!", displayName)
		printSuccessMessage()
	}()

	backoff := wait.Backoff{
		Steps:    60,
		Duration: 5 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
	}

	timeoutDuration := totalTimeoutDuration(backoff)
	fmt.Printf("Waiting for %s to be ready. This may take up to %v\n", displayName, timeoutDuration)

	progressBar.Infof("Waiting for %s to be ready: 0/3 resources ready", displayName)

	// Use the wait.ExponentialBackoff function to wait for the Deployment to be ready.
	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		readyCount := 0

		kotsadmReady, err := isDeploymentReady(clientset, namespace, "kotsadm")
		if err != nil {
			progressBar.Errorf("Error checking status of kotsadm: %v", err)
			return false, nil
		}
		if kotsadmReady {
			readyCount++
		}

		rqliteReady, err := isStatefulSetReady(clientset, namespace, "kotsadm-rqlite")
		if err != nil {
			progressBar.Errorf("Error checking status of kotsadm-rqlite: %v", err)
			return false, nil
		}
		if rqliteReady {
			readyCount++
		}

		minioReady, err := isStatefulSetReady(clientset, namespace, "kotsadm-minio")
		if err != nil {
			progressBar.Errorf("Error checking status of kotsadm-minio: %v", err)
			return false, nil
		}
		if minioReady {
			readyCount++
		}

		progressBar.Infof(
			"Waiting for %s to be ready: %d/3 resources ready",
			displayName,
			readyCount,
		)

		if readyCount == 3 {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for %s to be ready", displayName)
	}

	return nil
}

func totalTimeoutDuration(backoff wait.Backoff) time.Duration {
	total := time.Duration(0)
	duration := backoff.Duration
	for i := 0; i < backoff.Steps; i++ {
		total += duration
		duration = time.Duration(float64(duration) * backoff.Factor)
	}
	return total
}

func isDeploymentReady(clientset *kubernetes.Clientset, namespace, deploymentName string) (bool, error) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	// Check if all replicas are ready.
	return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas, nil
}

func isStatefulSetReady(clientset *kubernetes.Clientset, namespace, statefulSetName string) (bool, error) {
	statefulset, err := clientset.AppsV1().StatefulSets(namespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	// Check if all replicas are ready.
	return statefulset.Status.ReadyReplicas == *statefulset.Spec.Replicas, nil
}

func printSuccessMessage() {
	successColor := "\033[32m"
	colorReset := "\033[0m"

	ipaddr := defaults.TryDiscoverPublicIP()
	if ipaddr == "" {
		var err error
		ipaddr, err = defaults.PreferredNodeIPAddress()
		if err != nil {
			fmt.Println(fmt.Errorf("unable to determine node IP address: %w", err))
			ipaddr = "NODE-IP-ADDRESS"
		}
	}
	nodePort := helmValues["service"].(map[string]interface{})["nodePort"]
	successMessage := fmt.Sprintf("Admin Console accessible at: %shttps://%s:%v%s", successColor, ipaddr, nodePort, colorReset)
	fmt.Println(successMessage)
}
