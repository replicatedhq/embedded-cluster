package util

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func WaitForAdminConsoleReady(configPath, namespace, deploymentName, displayName string, backoff wait.Backoff) error {
	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// Create a dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Define the GVK (Group-Version-Kind) for the custom resource
	K0sChartsGVR := schema.GroupVersionResource{
		Group:    "helm.k0sproject.io", // replace with your CR's group
		Version:  "v1beta1",            // replace with your CR's version
		Resource: "charts",             // replace with your CR's plural resource name
	}

	k0sChartsNamespace := "kube-system"                            // replace with the namespace of your CR
	k0sChartsCustomResourceName := "k0s-addon-chart-admin-console" // replace with the name of your CR

	// Fetch the custom resource using the dynamic client
	cr, err := dynamicClient.Resource(K0sChartsGVR).Namespace(k0sChartsNamespace).Get(context.TODO(), k0sChartsCustomResourceName, v1.GetOptions{})
	//cr, err := dynamicClient.Resource(K0sChartsGVR).Namespace(k0sChartsNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	data, err := json.Marshal(cr)
	if err != nil {
		panic(err)
	}

	var testVar v1beta1.Chart
	err = json.Unmarshal(data, &testVar)
	if err != nil {
		panic(err)
	}

	fmt.Println(testVar)

	// Function to check if all replicas of the Deployment are ready.
	isDeploymentReady := func() (bool, error) {
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check if all replicas are ready.
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas, nil
	}

	if displayName == "" {
		displayName = deploymentName
	}

	progressBar := pb.Start()
	defer func() {
		progressBar.Closef("%s is ready!", displayName)
	}()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	counter := 1

	timeoutDuration := totalTimeoutDuration(backoff)
	fmt.Printf("Waiting for %s to be ready. This may take up to %v\n", displayName, timeoutDuration)

	progressBar.Infof("1/n Waiting for %s to be ready...", displayName)

	// Use the wait.ExponentialBackoff function to wait for the Deployment to be ready.
	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		ready, err := isDeploymentReady()
		if err != nil {
			progressBar.Errorf("Error checking status of %s", displayName)
			return false, nil
		}

		if ready {
			return true, nil
		}

		progressBar.Infof(
			"%d/n Waiting for %s to be ready...",
			counter,
			displayName,
		)
		counter++

		return false, nil
	})
	if err != nil {
		return fmt.Errorf("Timed out waiting for %s to be ready", displayName)
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
