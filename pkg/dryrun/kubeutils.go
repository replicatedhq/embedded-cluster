package dryrun

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KubeUtils struct{}

var _ kubeutils.KubeUtilsInterface = (*KubeUtils)(nil)

func (k *KubeUtils) WaitForNamespace(ctx context.Context, cli client.Client, ns string, opts *kubeutils.WaitOptions) error {
	return nil
}

func (k *KubeUtils) WaitForDeployment(ctx context.Context, cli client.Client, ns, name string, opts *kubeutils.WaitOptions) error {
	return nil
}

func (k *KubeUtils) WaitForStatefulset(ctx context.Context, cli client.Client, ns, name string, opts *kubeutils.WaitOptions) error {
	return nil
}

func (k *KubeUtils) WaitForDaemonset(ctx context.Context, cli client.Client, ns, name string, opts *kubeutils.WaitOptions) error {
	return nil
}

func (k *KubeUtils) WaitForService(ctx context.Context, cli client.Client, ns, name string, opts *kubeutils.WaitOptions) error {
	return nil
}

func (k *KubeUtils) WaitForJob(ctx context.Context, cli client.Client, ns, name string, completions int32, opts *kubeutils.WaitOptions) error {
	return nil
}

func (k *KubeUtils) WaitForPodComplete(ctx context.Context, cli client.Client, ns, name string, opts *kubeutils.WaitOptions) (*corev1.Pod, error) {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
		},
	}, nil
}

func (k *KubeUtils) WaitForPodDeleted(ctx context.Context, cli client.Client, ns, name string, opts *kubeutils.WaitOptions) error {
	return nil
}

func (k *KubeUtils) WaitForInstallation(ctx context.Context, cli client.Client, writer *spinner.MessageWriter) error {
	return nil
}

func (k *KubeUtils) WaitForNodes(ctx context.Context, cli client.Client) error {
	return nil
}

func (k *KubeUtils) WaitForNode(ctx context.Context, cli client.Client, name string, isWorker bool) error {
	return nil
}

func (k *KubeUtils) IsNamespaceReady(ctx context.Context, cli client.Client, ns string) (bool, error) {
	return true, nil
}

func (k *KubeUtils) IsDeploymentReady(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
	return true, nil
}

func (k *KubeUtils) IsStatefulSetReady(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
	return true, nil
}

func (k *KubeUtils) IsDaemonsetReady(ctx context.Context, cli client.Client, ns, name string) (bool, error) {
	return true, nil
}

func (k *KubeUtils) WaitForKubernetes(ctx context.Context, cli client.Client) <-chan error {
	errCh := make(chan error)
	close(errCh)
	return errCh
}

func (k *KubeUtils) WaitForCRDToBeReady(ctx context.Context, cli client.Client, name string) error {
	return nil
}

func (k *KubeUtils) KubeClient() (client.Client, error) {
	return KubeClient()
}

func (k *KubeUtils) MetadataClient() (metadata.Interface, error) {
	return MetadataClient()
}

func (k *KubeUtils) GetClientset() (kubernetes.Interface, error) {
	return GetClientSet()
}
