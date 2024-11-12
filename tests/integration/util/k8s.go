package util

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/yaml"
)

func KubectlApply(t *testing.T, kubeconfig string, namespace string, file string) {
	t.Logf("applying %s", file)
	out, err := exec.Command("kubectl", "--kubeconfig", kubeconfig, "apply", "-f", file, "-n", namespace).CombinedOutput()
	if err != nil {
		t.Logf("output: %s", out)
		t.Fatalf("failed to apply %s: %s", file, err)
	}
	t.Logf("applied %s", file)
}

func WaitForPodComplete(t *testing.T, kubeconfig string, namespace string, name string, timeout time.Duration) {
	t.Logf("waiting for pod %s:%s to be succeeded", namespace, name)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		return isPodSucceeded(kubeconfig, namespace, name)
	})
	if err != nil {
		t.Fatalf("failed to wait for pod %s:%s: %s", namespace, name, err)
	}
	t.Logf("pod %s:%s is succeeded", namespace, name)
}

func isPodSucceeded(kubeconfig string, namespace string, name string) (bool, error) {
	cmd := exec.Command(
		"kubectl", "--kubeconfig", kubeconfig, "get", "pod", name, "-n", namespace,
		"-o", "jsonpath={.status.phase}",
	)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	if string(out) == "Succeeded" {
		return true, nil
	}
	return false, nil
}

func WaitForDeployment(t *testing.T, kubeconfig string, namespace string, name string, replicas int, timeout time.Duration) {
	t.Logf("waiting for deployment %s:%s to be ready", namespace, name)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		return isDeploymentReady(kubeconfig, namespace, name, replicas)
	})
	if err != nil {
		t.Fatalf("failed to wait for deployment %s:%s: %s", namespace, name, err)
	}
	t.Logf("deployment %s:%s is ready", namespace, name)
}

func isDeploymentReady(kubeconfig string, namespace string, name string, replicas int) (bool, error) {
	cmd := exec.Command(
		"kubectl", "--kubeconfig", kubeconfig, "get", "deployment", name, "-n", namespace,
		"-o", "jsonpath={.status.readyReplicas}|{.status.updatedReplicas}|{.status.availableReplicas}|{.status.replicas}",
	)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return string(out) == fmt.Sprintf("%d|%d|%d|%d", replicas, replicas, replicas, replicas), nil
}

func GetDeployment(t *testing.T, kubeconfig string, namespace string, name string) *appsv1.Deployment {
	cmd := exec.Command(
		"kubectl", "--kubeconfig", kubeconfig, "get", "deployment", name, "-n", namespace,
		"-o", "yaml",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get deployment %s:%s: %v", namespace, name, err)
	}
	var resource appsv1.Deployment
	err = yaml.Unmarshal(out, &resource)
	if err != nil {
		t.Fatalf("failed to unmarshal deployment %s:%s: %v", namespace, name, err)
	}
	return &resource
}

func WaitForStorageClass(t *testing.T, kubeconfig string, name string, timeout time.Duration) {
	t.Logf("waiting for storageclass %s", name)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		cmd := exec.Command(
			"kubectl", "--kubeconfig", kubeconfig, "get", "sc", name,
		)
		err := cmd.Run()
		if err == nil {
			return true, nil
		}
		t.Logf("failed to get storageclass %s: %s", name, err)
		return false, nil
	})
	if err != nil {
		t.Fatalf("failed to wait for storageclass %s: %s", name, err)
	}
	t.Logf("storageclass %s exists", name)
}
