package util

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func WaitForAdminConsoleReady(configPath, namespace, deploymentName, displayName string, backoff wait.Backoff) error {
	clientset, err := kubeClient(configPath)
	if err != nil {
		return err
	}

	/*k0sChartsNamespace := "kube-system"                            // replace with the namespace of your CR
	k0sChartsCustomResourceName := "k0s-addon-chart-admin-console" // replace with the name of your CR

	var chartInstance v1beta1.Chart
	err = clientset.Get(context.TODO(), client.ObjectKey{Name: k0sChartsCustomResourceName, Namespace: k0sChartsNamespace}, &chartInstance)
	if err != nil {
		return err
	}

	fmt.Println(chartInstance.Status)*/

	// Function to check if all replicas of the Deployment are ready.
	isDeploymentReady := func() (bool, error) {
		deployment := &appsv1.Deployment{}
		err := clientset.Get(context.TODO(), client.ObjectKey{Name: deploymentName, Namespace: namespace}, deployment)
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

func kubeClient(configPath string) (client.Client, error) {
	/*k8slogger := zap.New(func(o *zap.Options) {
		o.DestWriter = io.Discard
	})
	log.SetLogger(k8slogger)*/

	// Add appsv1 scheme to the default client-go scheme
	if err := appsv1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}

	if err := v1beta1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return nil, err
	}

	// When constructing the client, use the scheme that includes appsv1
	return client.New(cfg, client.Options{Scheme: scheme.Scheme})
}
