package openebs

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	storageProvisionerAnnotationKey               = "volume.kubernetes.io/storage-provisioner"
	storageProvisionerOpenebsLocalAnnotationValue = "openebs.io/local"
	selectedNodeAnnotationKey                     = "volume.kubernetes.io/selected-node"
)

// CleanupStatefulPods checks if any pods with pvcs in a pending state were running on nodes that
// no longer exist and deletes them.
func CleanupStatefulPods(ctx context.Context, cli client.Client) error {
	stuckPVCs, err := findStuckPVCs(ctx, cli)
	if err != nil {
		return fmt.Errorf("find stuck pvcs: %w", err)
	}

	pvcsByNamespace := make(map[string][]corev1.PersistentVolumeClaim)
	for _, pvc := range stuckPVCs {
		pvcsByNamespace[pvc.Namespace] = append(pvcsByNamespace[pvc.Namespace], pvc)
	}

	for namespace, pvcs := range pvcsByNamespace {
		err := deletePendingPodsWithPVCsInNamespace(ctx, cli, namespace, pvcs)
		if err != nil {
			return fmt.Errorf("delete pods with pvcs in namespace %s: %w", namespace, err)
		}
	}

	for _, pvc := range stuckPVCs {
		err = deletePVC(ctx, cli, pvc)
		if err != nil {
			return fmt.Errorf("delete stuck pvc %s: %w", pvc.Name, err)
		}
	}

	return nil
}

func findStuckPVCs(ctx context.Context, cli client.Client) ([]corev1.PersistentVolumeClaim, error) {
	var pvcs corev1.PersistentVolumeClaimList
	err := cli.List(ctx, &pvcs)
	if err != nil {
		return nil, fmt.Errorf("list pvcs: %w", err)
	}

	var nodes corev1.NodeList
	err = cli.List(ctx, &nodes)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	var stuck []corev1.PersistentVolumeClaim
	for _, pvc := range pvcs.Items {
		if pvc.Annotations == nil {
			continue
		}
		if pvc.Annotations[storageProvisionerAnnotationKey] != storageProvisionerOpenebsLocalAnnotationValue {
			continue
		}

		if isNodeGone(nodes, pvc.Annotations[selectedNodeAnnotationKey]) {
			stuck = append(stuck, pvc)
		}
	}

	return stuck, nil
}

func deletePendingPodsWithPVCsInNamespace(ctx context.Context, cli client.Client, namespace string, pvcs []corev1.PersistentVolumeClaim) error {
	log := ctrl.LoggerFrom(ctx)

	var pods corev1.PodList
	err := cli.List(ctx, &pods, client.InNamespace(namespace))
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}

	for _, pod := range pods.Items {
		// This pod is likely stuck due to SchedulerError 'nodeinfo not found for node name "NODE_NAME"'
		if pod.Status.Phase != corev1.PodPending {
			continue
		}

		if !isPodMountingPVC(pod, pvcs) {
			continue
		}

		// Stateful pod was running on a node that is no longer in the cluster.
		// Remove the pod and let the controller recreate it.
		err = cli.Delete(ctx, &pod)
		if err != nil {
			return fmt.Errorf("delete pod %s: %w", pod.Name, err)
		} else {
			log.Info("Deleted stateful pod", "name", pod.Name, "namespace", pod.Namespace, "node", pod.Spec.NodeName)
		}
	}

	return nil
}

func deletePVC(ctx context.Context, cli client.Client, pvc corev1.PersistentVolumeClaim) error {
	log := ctrl.LoggerFrom(ctx)

	var pv corev1.PersistentVolume
	err := cli.Get(ctx, client.ObjectKey{Name: pvc.Spec.VolumeName}, &pv)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("get pv %s: %w", pvc.Spec.VolumeName, err)
	}

	if pv.ObjectMeta.Name != "" {
		err = cli.Delete(ctx, &pv)
		if err != nil {
			return fmt.Errorf("delete pv %s: %w", pv.Name, err)
		}

		log.Info("Deleted pv", "name", pv.Name)
	}

	err = cli.Delete(ctx, &pvc)
	if err != nil {
		return fmt.Errorf("delete pvc %s: %w", pvc.Name, err)
	}

	log.Info("Deleted pvc", "name", pvc.Name, "namespace", pvc.Namespace)

	return nil
}

func isNodeGone(nodes corev1.NodeList, nodeName string) bool {
	if nodeName == "" {
		return false
	}

	for _, node := range nodes.Items {
		if nodeName == node.Name {
			return false
		}
	}

	return true
}

func isPodMountingPVC(pod corev1.Pod, pvcs []corev1.PersistentVolumeClaim) bool {
	for _, pvc := range pvcs {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				return true
			}
		}
	}

	return false
}
