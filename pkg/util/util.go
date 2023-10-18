package util

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func WaitForDeploymentReady(namespace, deploymentName string) error {
	// Load Kubernetes config to create a clientset.
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// Function to check if all replicas of the Deployment are ready.
	isDeploymentReady := func() (bool, error) {
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check if all replicas are ready.
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas, nil
	}

	// Define the backoff and timeout settings for retries.
	backoff := wait.Backoff{
		Steps:    10,
		Duration: 10 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
	}

	// Use the wait.ExponentialBackoff function to wait for the Deployment to be ready.
	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		ready, err := isDeploymentReady()
		if err != nil {
			fmt.Printf("Error checking Deployment status: %v\n", err)
			return false, nil
		}

		if ready {
			fmt.Printf("Deployment %s in namespace %s is ready!\n", deploymentName, namespace)
			return true, nil
		}

		fmt.Printf("Deployment %s in namespace %s is not yet ready. Retrying...\n", deploymentName, namespace)
		return false, nil
	})

	if err != nil {
		return fmt.Errorf("Timed out waiting for Deployment %s in namespace %s to be ready", deploymentName, namespace)
	}

	return nil
}
