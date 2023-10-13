/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/replicatedhq/helmvm-operator/api/v2alpha1"
	"github.com/replicatedhq/helmvm-operator/pkg/metrics"
)

// requeueAfter is our default interval for requeueing. If nothing has changed with the
// cluster nodes or the Installation object we will reconcile once every requeueAfter
// interval.
var requeueAfter = time.Hour

// InstallationReconciler reconciles a Installation object
type InstallationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NodeHasChanged returns true if the node configuration has changed when compared to
// the node information we keep in the installation status. Returns a bool indicating
// if a change was detected and a bool indicating if the node is new (not seen yet).
func (r *InstallationReconciler) NodeHasChanged(in *v2alpha1.Installation, ev metrics.NodeEvent) (bool, bool, error) {
	for _, nodeStatus := range in.Status.NodesStatus {
		if nodeStatus.Name != ev.NodeName {
			continue
		}
		eventHash, err := ev.Hash()
		if err != nil {
			return false, false, err
		}
		return nodeStatus.Hash != eventHash, false, nil
	}
	return true, true, nil
}

// UpdateNodeStatus updates the node status in the Installation object status.
func (r *InstallationReconciler) UpdateNodeStatus(in *v2alpha1.Installation, ev metrics.NodeEvent) error {
	hash, err := ev.Hash()
	if err != nil {
		return err
	}
	for i, nodeStatus := range in.Status.NodesStatus {
		if nodeStatus.Name != ev.NodeName {
			continue
		}
		in.Status.NodesStatus[i].Hash = hash
		return nil
	}
	in.Status.NodesStatus = append(in.Status.NodesStatus, v2alpha1.NodeStatus{Name: ev.NodeName, Hash: hash})
	return nil
}

// ReconcileInstallation is the function that actually reconciles the Installation object.
// Metrics events (call back home) are generated here.
func (r *InstallationReconciler) ReconcileInstallation(ctx context.Context, in *v2alpha1.Installation) error {
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}
	seen := map[string]bool{}
	needsUpdate := false
	for _, node := range nodes.Items {
		seen[node.Name] = true
		event := metrics.NodeEventFromNode(in.Spec.ClusterID, node)
		changed, isnew, err := r.NodeHasChanged(in, event)
		if err != nil {
			return fmt.Errorf("failed to check if node has changed: %w", err)
		} else if !changed {
			continue
		}
		needsUpdate = true
		if err := r.UpdateNodeStatus(in, event); err != nil {
			return fmt.Errorf("failed to update node status: %w", err)
		}
		if in.Spec.AirGap {
			continue
		}
		if isnew {
			if err := metrics.NotifyNodeAdded(ctx, in.Spec.MetricsBaseURL, event); err != nil {
				return fmt.Errorf("failed to notify node added: %w", err)
			}
			continue
		}
		if err := metrics.NotifyNodeUpdated(ctx, in.Spec.MetricsBaseURL, event); err != nil {
			return fmt.Errorf("failed to notify node updated: %w", err)
		}
	}
	trimmed := []v2alpha1.NodeStatus{}
	for _, nodeStatus := range in.Status.NodesStatus {
		if _, ok := seen[nodeStatus.Name]; ok {
			trimmed = append(trimmed, nodeStatus)
			continue
		}
		needsUpdate = true
		rmevent := metrics.NodeRemovedEvent{ClusterID: in.Spec.ClusterID, NodeName: nodeStatus.Name}
		if in.Spec.AirGap {
			continue
		}
		if err := metrics.NotifyNodeRemoved(ctx, in.Spec.MetricsBaseURL, rmevent); err != nil {
			return fmt.Errorf("failed to notify node removed: %w", err)
		}
	}
	log := ctrl.LoggerFrom(ctx)
	sort.SliceStable(trimmed, func(i, j int) bool { return trimmed[i].Name < trimmed[j].Name })
	in.Status.NodesStatus = trimmed
	if !needsUpdate {
		log.Info("No node changes detected")
		return nil
	}
	log.Info("Node changes detected, updating installation status")
	if err := r.Status().Update(ctx, in); err != nil {
		return fmt.Errorf("failed to update installation status: %w", err)
	}
	return nil
}

//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=cluster.kurl.sh,resources=installations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.kurl.sh,resources=installations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.kurl.sh,resources=installations/finalizers,verbs=update

// Reconcile reconcile the installation object.
func (r *InstallationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	var installs v2alpha1.InstallationList
	if err := r.List(ctx, &installs); err != nil {
		return ctrl.Result{}, err
	}
	log.Info("Reconciling installation")
	if len(installs.Items) == 0 {
		log.Info("No installation found, reconciliation ended")
		return ctrl.Result{}, nil
	}
	if len(installs.Items) > 1 {
		log.Error(fmt.Errorf("multiple installations found"), "Multiple installations found")
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
	if installs.Items[0].Spec.ClusterID == "" {
		log.Info("No cluster ID, reconciliation ended")
		return ctrl.Result{}, nil
	}
	if err := r.ReconcileInstallation(ctx, &installs.Items[0]); err != nil {
		return ctrl.Result{}, err
	}
	log.Info("Installation reconciliation ended")
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InstallationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v2alpha1.Installation{}).
		Watches(&corev1.Node{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
