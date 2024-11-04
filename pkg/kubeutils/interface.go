package kubeutils

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var kb KubeUtilsInterface

func init() {
	Set(&KubeUtils{})
}

func Set(_kb KubeUtilsInterface) {
	kb = _kb
}

type KubeUtilsInterface interface {
	WaitForNamespace(ctx context.Context, cli client.Client, ns string) error
	WaitForDeployment(ctx context.Context, cli client.Client, ns, name string) error
	WaitForDaemonset(ctx context.Context, cli client.Client, ns, name string) error
	WaitForService(ctx context.Context, cli client.Client, ns, name string) error
	WaitForInstallation(ctx context.Context, cli client.Client, writer *spinner.MessageWriter) error
	WaitForHAInstallation(ctx context.Context, cli client.Client) error
	WaitForNodes(ctx context.Context, cli client.Client) error
	WaitForControllerNode(ctx context.Context, kcli client.Client, name string) error
	WaitForJob(ctx context.Context, cli client.Client, ns, name string, maxSteps int, completions int32) error
	IsNamespaceReady(ctx context.Context, cli client.Client, ns string) (bool, error)
	IsDeploymentReady(ctx context.Context, cli client.Client, ns, name string) (bool, error)
	IsStatefulSetReady(ctx context.Context, cli client.Client, ns, name string) (bool, error)
	IsDaemonsetReady(ctx context.Context, cli client.Client, ns, name string) (bool, error)
	IsJobComplete(ctx context.Context, cli client.Client, ns, name string, completions int32) (bool, error)
	WaitForKubernetes(ctx context.Context, cli client.Client) <-chan error
	WaitAndMarkInstallation(ctx context.Context, cli client.Client, name string, state string) error
	KubeClient() (client.Client, error)
}

// Convenience functions

func WaitForNamespace(ctx context.Context, cli client.Client, ns string) error {
	return kb.WaitForNamespace(ctx, cli, ns)
}

func WaitForDeployment(ctx context.Context, cli client.Client, ns, name string) error {
	return kb.WaitForDeployment(ctx, cli, ns, name)
}

func WaitForDaemonset(ctx context.Context, cli client.Client, ns, name string) error {
	return kb.WaitForDaemonset(ctx, cli, ns, name)
}

func WaitForService(ctx context.Context, cli client.Client, ns, name string) error {
	return kb.WaitForService(ctx, cli, ns, name)
}

func WaitForInstallation(ctx context.Context, cli client.Client, writer *spinner.MessageWriter) error {
	return kb.WaitForInstallation(ctx, cli, writer)
}

func WaitForHAInstallation(ctx context.Context, cli client.Client) error {
	return kb.WaitForHAInstallation(ctx, cli)
}

func WaitForNodes(ctx context.Context, cli client.Client) error {
	return kb.WaitForNodes(ctx, cli)
}

func WaitForControllerNode(ctx context.Context, kcli client.Client, name string) error {
	return kb.WaitForControllerNode(ctx, kcli, name)
}

func WaitForJob(ctx context.Context, cli client.Client, ns, name string, maxSteps int, completions int32) error {
	return kb.WaitForJob(ctx, cli, ns, name, maxSteps, completions)
}

func IsNamespaceReady(ctx context.Context, cli client.Client, ns string) (bool, error) {
	return kb.IsNamespaceReady(ctx, cli, ns)
}

func IsDeploymentReady(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
	return kb.IsDeploymentReady(ctx, cli, ns, name)
}

func IsStatefulSetReady(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
	return kb.IsStatefulSetReady(ctx, cli, ns, name)
}

func IsDaemonsetReady(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
	return kb.IsDaemonsetReady(ctx, cli, ns, name)
}

func IsJobComplete(ctx context.Context, cli client.Client, ns, name string, completions int32) (bool, error) {
	return kb.IsJobComplete(ctx, cli, ns, name, completions)
}

func WaitForKubernetes(ctx context.Context, cli client.Client) <-chan error {
	return kb.WaitForKubernetes(ctx, cli)
}

func WaitAndMarkInstallation(ctx context.Context, cli client.Client, name string, state string) error {
	return kb.WaitAndMarkInstallation(ctx, cli, name, state)
}

func KubeClient() (client.Client, error) {
	return kb.KubeClient()
}
