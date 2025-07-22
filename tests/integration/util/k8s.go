package util

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"
)

// SetupCtrlLogging returns a function that can be used to capture the logs from the controller-runtime package.
func SetupCtrlLogging(t *testing.T) {
	pr, pw := io.Pipe()
	k8slogger := ctrlzap.New(func(o *ctrlzap.Options) {
		o.DestWriter = pw
	})
	ctrllog.SetLogger(k8slogger)

	go func() {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			t.Log(scanner.Text())
		}
	}()
}

func CtrlClient(t *testing.T, kubeconfig string) client.Client {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("failed to build config from flags: %s", err)
	}

	// Create Kubernetes clients
	kcli, err := client.New(config, client.Options{})
	if err != nil {
		t.Fatalf("failed to create kubernetes client: %s", err)
	}
	return kcli
}

func KubeClient(t *testing.T, kubeconfig string) kubernetes.Interface {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("failed to build config from flags: %s", err)
	}
	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("failed to create kubernetes client: %s", err)
	}
	return kclient
}

func MetadataClient(t *testing.T, kubeconfig string) metadata.Interface {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("failed to build config from flags: %s", err)
	}
	mcli, err := metadata.NewForConfig(config)
	if err != nil {
		t.Fatalf("failed to create metadata client: %s", err)
	}
	return mcli
}

func KubectlApply(t *testing.T, kubeconfig string, namespace string, file string) {
	t.Logf("applying %s", filepath.Base(file))
	out, err := exec.Command(
		"kubectl", "--kubeconfig", kubeconfig, "apply", "-f", file, "-n", namespace,
	).CombinedOutput()
	if err != nil {
		t.Logf("output: %s", out)
		t.Fatalf("failed to apply %s: %s", file, err)
	}
	t.Logf("applied %s", filepath.Base(file))
}

func WaitForPodComplete(t *testing.T, kubeconfig string, namespace string, name string, timeout time.Duration) {
	t.Logf("waiting for pod %s:%s to be succeeded", namespace, name)
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		return isPodSucceeded(kubeconfig, namespace, name)
	})
	if err != nil {
		K8sDescribe(t, kubeconfig, namespace, "pod", name)
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
	} else if string(out) == "Failed" {
		return false, fmt.Errorf("pod failed")
	}
	return false, nil
}

func WaitForDeployment(t *testing.T, kubeconfig string, namespace string, name string, replicas int, timeout time.Duration) {
	t.Logf("waiting for deployment %s:%s to be ready", namespace, name)
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
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
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("failed to get deployment %s:%s: %v", namespace, name, string(exitErr.Stderr))
		}
		t.Fatalf("failed to get deployment %s:%s: %v", namespace, name, err)
	}
	var resource appsv1.Deployment
	err = yaml.Unmarshal(out, &resource)
	if err != nil {
		t.Fatalf("failed to unmarshal deployment %s:%s: %v", namespace, name, err)
	}
	return &resource
}

func GetDaemonSet(t *testing.T, kubeconfig string, namespace string, name string) *appsv1.DaemonSet {
	cmd := exec.Command(
		"kubectl", "--kubeconfig", kubeconfig, "get", "daemonset", name, "-n", namespace,
		"-o", "yaml",
	)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("failed to get daemonset %s:%s: %v", namespace, name, string(exitErr.Stderr))
		}
		t.Fatalf("failed to get daemonset %s:%s: %v", namespace, name, err)
	}
	var resource appsv1.DaemonSet
	err = yaml.Unmarshal(out, &resource)
	if err != nil {
		t.Fatalf("failed to unmarshal daemonset %s:%s: %v", namespace, name, err)
	}
	return &resource
}

func WaitForStorageClass(t *testing.T, kubeconfig string, name string, timeout time.Duration) {
	t.Logf("waiting for storageclass %s", name)
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
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

func K8sDescribe(t *testing.T, kubeconfig string, namespace string, kind string, name string) {
	cmd := exec.Command(
		"kubectl", "--kubeconfig", kubeconfig, "describe", kind, name, "-n", namespace,
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to describe %s %s in namespace %s: %s", kind, name, namespace, err)
	}
	t.Logf("kubectl -n %s describe %s %s:\n%s", namespace, kind, name, out)
}
