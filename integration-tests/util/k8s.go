package util

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

func WaitForDeployment(t *testing.T, namespace string, name string, replicas int, timeout time.Duration) {
	t.Logf("waiting for deployment %s:%s to be ready", namespace, name)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		ready := IsDeploymentReady(t, namespace, name, replicas)
		return ready, nil
	})
	if err != nil {
		t.Fatalf("failed to wait for deployment %s:%s: %s", namespace, name, err)
	}
	t.Logf("deployment %s:%s is ready", namespace, name)
}

func IsDeploymentReady(t *testing.T, namespace string, name string, replicas int) bool {
	cmd := exec.Command(
		"kubectl", "get", "deployment", name, "-n", namespace,
		"-o", "jsonpath={.status.readyReplicas}|{.status.updatedReplicas}|{.status.availableReplicas}|{.status.replicas}",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get deployment status: %s", err)
	}
	return string(out) == fmt.Sprintf("%d|%d|%d|%d", replicas, replicas, replicas, replicas)
}
