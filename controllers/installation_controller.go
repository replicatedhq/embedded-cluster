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
	"strings"
	"time"

	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apcore "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	"github.com/k0sproject/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/autopilot"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/release"
)

// requeueAfter is our default interval for requeueing. If nothing has changed with the
// cluster nodes or the Installation object we will reconcile once every requeueAfter
// interval.
var requeueAfter = time.Hour

// NodeEventsBatch is a batch of node events, meant to be gathered at a given
// moment in time and send later on to the metrics server.
type NodeEventsBatch struct {
	NodesAdded   []metrics.NodeEvent
	NodesUpdated []metrics.NodeEvent
	NodesRemoved []metrics.NodeRemovedEvent
}

// InstallationReconciler reconciles a Installation object
type InstallationReconciler struct {
	client.Client
	Discovery discovery.DiscoveryInterface
	Scheme    *runtime.Scheme
}

// NodeHasChanged returns true if the node configuration has changed when compared to
// the node information we keep in the installation status. Returns a bool indicating
// if a change was detected and a bool indicating if the node is new (not seen yet).
func (r *InstallationReconciler) NodeHasChanged(in *v1beta1.Installation, ev metrics.NodeEvent) (bool, bool, error) {
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
func (r *InstallationReconciler) UpdateNodeStatus(in *v1beta1.Installation, ev metrics.NodeEvent) error {
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
	in.Status.NodesStatus = append(in.Status.NodesStatus, v1beta1.NodeStatus{Name: ev.NodeName, Hash: hash})
	return nil
}

// ReconcileNodeStatuses reconciles the node statuses in the Installation object status. Installation
// is not updated remotely but only in the memory representation of the object (aka caller must save
// the object after the call). This function returns a batch of events that need to be sent back to
// the metrics endpoint, these events represent changes in the node statuses.
func (r *InstallationReconciler) ReconcileNodeStatuses(ctx context.Context, in *v1beta1.Installation) (*NodeEventsBatch, error) {
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	batch := &NodeEventsBatch{}
	seen := map[string]bool{}
	for _, node := range nodes.Items {
		seen[node.Name] = true
		event := metrics.NodeEventFromNode(in.Spec.ClusterID, node)
		changed, isnew, err := r.NodeHasChanged(in, event)
		if err != nil {
			return nil, fmt.Errorf("failed to check if node has changed: %w", err)
		} else if !changed {
			continue
		}
		if err := r.UpdateNodeStatus(in, event); err != nil {
			return nil, fmt.Errorf("failed to update node status: %w", err)
		}
		if isnew {
			batch.NodesAdded = append(batch.NodesAdded, event)
			continue
		}
		batch.NodesUpdated = append(batch.NodesUpdated, event)
	}
	trimmed := []v1beta1.NodeStatus{}
	for _, nodeStatus := range in.Status.NodesStatus {
		if _, ok := seen[nodeStatus.Name]; ok {
			trimmed = append(trimmed, nodeStatus)
			continue
		}
		rmevent := metrics.NodeRemovedEvent{
			ClusterID: in.Spec.ClusterID, NodeName: nodeStatus.Name,
		}
		batch.NodesRemoved = append(batch.NodesRemoved, rmevent)
	}
	sort.SliceStable(trimmed, func(i, j int) bool { return trimmed[i].Name < trimmed[j].Name })
	in.Status.NodesStatus = trimmed
	return batch, nil
}

// ReportNodesChanges reports node changes to the metrics endpoint.
func (r *InstallationReconciler) ReportNodesChanges(ctx context.Context, in *v1beta1.Installation, batch *NodeEventsBatch) {
	for _, ev := range batch.NodesAdded {
		if err := metrics.NotifyNodeAdded(ctx, in.Spec.MetricsBaseURL, ev); err != nil {
			ctrl.LoggerFrom(ctx).Error(err, "failed to notify node added")
		}
	}
	for _, ev := range batch.NodesUpdated {
		if err := metrics.NotifyNodeUpdated(ctx, in.Spec.MetricsBaseURL, ev); err != nil {
			ctrl.LoggerFrom(ctx).Error(err, "failed to notify node updated")
		}
	}
	for _, ev := range batch.NodesRemoved {
		if err := metrics.NotifyNodeRemoved(ctx, in.Spec.MetricsBaseURL, ev); err != nil {
			ctrl.LoggerFrom(ctx).Error(err, "failed to notify node removed")
		}
	}
}

// ReportInstallationChanges reports back to the metrics server if the installation status has changed.
func (r *InstallationReconciler) ReportInstallationChanges(ctx context.Context, before, after *v1beta1.Installation) {
	if len(before.Status.State) == 0 || before.Status.State == after.Status.State {
		return
	}
	var err error
	switch after.Status.State {
	case v1beta1.InstallationStateInstalling:
		err = metrics.NotifyUpgradeStarted(ctx, after.Spec.MetricsBaseURL, metrics.UpgradeStartedEvent{
			ClusterID: after.Spec.ClusterID,
			Version:   after.Spec.Config.Version,
		})
	case v1beta1.InstallationStateInstalled:
		err = metrics.NotifyUpgradeSucceeded(ctx, after.Spec.MetricsBaseURL, metrics.UpgradeSucceededEvent{
			ClusterID: after.Spec.ClusterID,
		})
	case v1beta1.InstallationStateFailed:
		err = metrics.NotifyUpgradeFailed(ctx, after.Spec.MetricsBaseURL, metrics.UpgradeFailedEvent{
			ClusterID: after.Spec.ClusterID,
			Reason:    after.Status.Reason,
		})
	}
	if err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to notify cluster installation status")
	}
}

// ReconcileK0sVersion reconciles the k0s version in the Installation object status. If the
// Installation spec.config points to a different version we start an upgrade Plan. If an
// upgrade plan already exists we make sure the installation status is updated with the
// latest plan status.
func (r *InstallationReconciler) ReconcileK0sVersion(ctx context.Context, in *v1beta1.Installation) error {
	if in.Spec.Config == nil || in.Spec.Config.Version == "" {
		in.Status.SetState(v1beta1.InstallationStateKubernetesInstalled, "")
		return nil
	}
	meta, err := release.MetadataFor(ctx, in.Spec.Config.Version, in.Spec.MetricsBaseURL)
	if err != nil {
		in.Status.SetState(v1beta1.InstallationStateFailed, err.Error())
		return nil
	}
	vinfo, err := r.Discovery.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get server version: %w", err)
	}
	runningVersion := vinfo.GitVersion
	if runningVersion == meta.Versions.Kubernetes {
		in.Status.SetState(v1beta1.InstallationStateKubernetesInstalled, "")
		return nil
	}
	running, err := version.NewVersion(runningVersion)
	if err != nil {
		reason := fmt.Sprintf("Invalid running version %s", runningVersion)
		in.Status.SetState(v1beta1.InstallationStateFailed, reason)
		return nil
	}
	desired, err := version.NewVersion(meta.Versions.Kubernetes)
	if err != nil {
		reason := fmt.Sprintf("Invalid desired version %s", in.Spec.Config.Version)
		in.Status.SetState(v1beta1.InstallationStateFailed, reason)
		return nil
	}
	if running.GreaterThan(desired) {
		in.Status.SetState(v1beta1.InstallationStateFailed, "Downgrades not supported")
		return nil
	}
	var plan apv1b2.Plan
	okey := client.ObjectKey{Name: "autopilot"}
	if err := r.Get(ctx, okey, &plan); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get upgrade plan: %w", err)
	} else if errors.IsNotFound(err) {
		if err := r.StartUpgrade(ctx, in); err != nil {
			return fmt.Errorf("failed to start upgrade: %w", err)
		}
		return nil
	}
	if plan.Spec.ID == in.Name {
		r.SetStateBasedOnPlan(in, plan)
		return nil
	}
	if !autopilot.HasThePlanEnded(plan) {
		reason := fmt.Sprintf("Another upgrade is in progress (%s)", plan.Spec.ID)
		in.Status.SetState(v1beta1.InstallationStateWaiting, reason)
		return nil
	}
	if err := r.Delete(ctx, &plan); err != nil {
		return fmt.Errorf("failed to delete previous upgrade plan: %w", err)
	}
	return nil
}

// ReconcileHelmCharts reconciles the helm charts from the Installation metadata with the clusterconfig object.
func (r *InstallationReconciler) ReconcileHelmCharts(ctx context.Context, in *v1beta1.Installation) error {
	if in.Spec.Config == nil || in.Spec.Config.Version == "" {
		if in.Status.State == v1beta1.InstallationStateKubernetesInstalled {
			in.Status.SetState(v1beta1.InstallationStateInstalled, "Installed")
		}
		return nil
	}

	// skip if the installer has already completed, failed or if the k0s upgrade is still in progress
	if in.Status.State == v1beta1.InstallationStateFailed ||
		in.Status.State == v1beta1.InstallationStateInstalled ||
		!in.Status.GetKubernetesInstalled() {
		return nil
	}

	log := ctrl.LoggerFrom(ctx)
	meta, err := release.MetadataFor(ctx, in.Spec.Config.Version, in.Spec.MetricsBaseURL)
	if err != nil {
		in.Status.SetState(v1beta1.InstallationStateHelmChartUpdateFailure, err.Error())
		return nil
	}

	// skip if the new release has no addon configs - this should not happen in production
	if meta.Configs == nil && in.Spec.Config.Extensions.Helm == nil {
		log.Info("addons", "configcheck", "no addons")
		if in.Status.State == v1beta1.InstallationStateKubernetesInstalled {
			in.Status.SetState(v1beta1.InstallationStateInstalled, "Installed")
		}
		return nil
	}

	combinedConfigs := mergeHelmConfigs(meta, in)

	// detect drift between the cluster config and the installer metadata
	var installedCharts k0shelm.ChartList
	if err := r.List(ctx, &installedCharts); err != nil {
		return fmt.Errorf("failed to list installed charts: %w", err)
	}
	chartErrors, chartDrift := detectChartDrift(combinedConfigs, installedCharts)

	// If all addons match their target version, mark installation as complete (or as failed, if there are errors)
	if !chartDrift {
		// If any chart has errors, update installer state and return
		if len(chartErrors) > 0 {
			chartErrorString := strings.Join(chartErrors, ",")
			chartErrorString = "failed to update helm charts: " + chartErrorString
			log.Info("chart errors", "errors", chartErrorString)
			if len(chartErrorString) > 1024 {
				chartErrorString = chartErrorString[:1024]
			}
			in.Status.SetState(v1beta1.InstallationStateHelmChartUpdateFailure, chartErrorString)
			return nil
		}
		in.Status.SetState(v1beta1.InstallationStateInstalled, "Addons upgraded")
		return nil
	}

	if in.Status.State != v1beta1.InstallationStateKubernetesInstalled {
		// after the first time we apply new helm charts, this will be set to InstallationStateAddonsInstalling
		// and we will not re-apply the charts to the k0s cluster config while waiting for those changes to propagate
		return nil
	}

	// fetch the current clusterconfig
	var clusterconfig k0sv1beta1.ClusterConfig
	if err := r.Get(ctx, client.ObjectKey{Name: "k0s", Namespace: "kube-system"}, &clusterconfig); err != nil {
		return fmt.Errorf("failed to get cluster config: %w", err)
	}

	finalChartList, err := generateDesiredCharts(meta, clusterconfig, combinedConfigs)
	if err != nil {
		return err
	}

	// Replace the current chart configs with the new chart configs
	combinedConfigs.Charts = finalChartList
	clusterconfig.Spec.Extensions.Helm = combinedConfigs
	in.Status.SetState(v1beta1.InstallationStateAddonsInstalling, "Installing addons")
	//Update the clusterconfig
	if err := r.Update(ctx, &clusterconfig); err != nil {
		return fmt.Errorf("failed to update cluster config: %w", err)
	}
	return nil
}

// SetStateBasedOnPlan sets the installation state based on the Plan state. For now we do not
// report anything fancy but we should consider reporting here a summary of how many nodes
// have been upgraded and how many are still pending.
func (r *InstallationReconciler) SetStateBasedOnPlan(in *v1beta1.Installation, plan apv1b2.Plan) {
	reason := autopilot.ReasonForState(plan)
	switch plan.Status.State {
	case "":
		in.Status.SetState(v1beta1.InstallationStateEnqueued, reason)
	case apcore.PlanIncompleteTargets:
		fallthrough
	case apcore.PlanInconsistentTargets:
		fallthrough
	case apcore.PlanRestricted:
		fallthrough
	case apcore.PlanWarning:
		fallthrough
	case apcore.PlanMissingSignalNode:
		fallthrough
	case apcore.PlanApplyFailed:
		in.Status.SetState(v1beta1.InstallationStateFailed, reason)
	case apcore.PlanSchedulable:
		fallthrough
	case apcore.PlanSchedulableWait:
		in.Status.SetState(v1beta1.InstallationStateInstalling, reason)
	case apcore.PlanCompleted:
		in.Status.SetState(v1beta1.InstallationStateKubernetesInstalled, reason)
	default:
		in.Status.SetState(v1beta1.InstallationStateFailed, reason)
	}
}

// DetermineUpgradeTargets makes sure that we are listing all the nodes in the autopilot plan.
func (r *InstallationReconciler) DetermineUpgradeTargets(ctx context.Context) (apv1b2.PlanCommandTargets, error) {
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		return apv1b2.PlanCommandTargets{}, fmt.Errorf("failed to list nodes: %w", err)
	}
	controllers := []string{}
	workers := []string{}
	for _, node := range nodes.Items {
		if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
			controllers = append(controllers, node.Name)
			continue
		}
		workers = append(workers, node.Name)
	}
	return apv1b2.PlanCommandTargets{
		Controllers: apv1b2.PlanCommandTarget{
			Discovery: apv1b2.PlanCommandTargetDiscovery{
				Static: &apv1b2.PlanCommandTargetDiscoveryStatic{Nodes: controllers},
			},
		},
		Workers: apv1b2.PlanCommandTarget{
			Discovery: apv1b2.PlanCommandTargetDiscovery{
				Static: &apv1b2.PlanCommandTargetDiscoveryStatic{Nodes: workers},
			},
		},
	}, nil
}

// StartUpgrade creates an autopilot plan to upgrade to version specified in spec.config.version.
func (r *InstallationReconciler) StartUpgrade(ctx context.Context, in *v1beta1.Installation) error {
	targets, err := r.DetermineUpgradeTargets(ctx)
	if err != nil {
		return fmt.Errorf("failed to determine upgrade targets: %w", err)
	}
	meta, err := release.MetadataFor(ctx, in.Spec.Config.Version, in.Spec.MetricsBaseURL)
	if err != nil {
		return fmt.Errorf("failed to get release bundle: %w", err)
	}

	k0surl := fmt.Sprintf(
		"%s/embedded-cluster-public-files/k0s-binaries/%s", in.Spec.MetricsBaseURL, meta.Versions.Kubernetes,
	)
	plan := apv1b2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
		},
		Spec: apv1b2.PlanSpec{
			Timestamp: "now",
			ID:        in.Name,
			Commands: []apv1b2.PlanCommand{
				{
					K0sUpdate: &apv1b2.PlanCommandK0sUpdate{
						Version: meta.Versions.Kubernetes,
						Targets: targets,
						Platforms: apv1b2.PlanPlatformResourceURLMap{
							"linux-amd64": {URL: k0surl, Sha256: meta.K0sSHA},
						},
					},
				},
			},
		},
	}
	if err := r.Create(ctx, &plan); err != nil {
		return fmt.Errorf("failed to create upgrade plan: %w", err)
	}
	in.Status.SetState(v1beta1.InstallationStateEnqueued, "")
	return nil
}

// CoalesceInstallations goes through all the installation objects and make sure that the
// status of the newest one is coherent with whole cluster status. Returns the newest
// installation object.
func (r *InstallationReconciler) CoalesceInstallations(
	ctx context.Context, items []v1beta1.Installation,
) *v1beta1.Installation {
	sort.SliceStable(items, func(i, j int) bool {
		return items[j].CreationTimestamp.Before(&items[i].CreationTimestamp)
	})
	if len(items) == 1 || len(items[0].Status.NodesStatus) > 0 {
		return &items[0]
	}
	for i := 1; i < len(items); i++ {
		if len(items[i].Status.NodesStatus) == 0 {
			continue
		}
		items[0].Status.NodesStatus = items[i].Status.NodesStatus
		break
	}
	return &items[0]
}

// DisableOldInstallations resets the old installation statuses keeping only the newest one with
// proper status set. This set the state for all old installations as "obsolete". We do not report
// errors back as this is not a critical operation, if we fail to update the status we will just
// retry on the next reconcile.
func (r *InstallationReconciler) DisableOldInstallations(ctx context.Context, items []v1beta1.Installation) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[j].CreationTimestamp.Before(&items[i].CreationTimestamp)
	})
	for _, in := range items[1:] {
		in.Status.NodesStatus = nil
		in.Status.SetState(
			v1beta1.InstallationStateObsolete,
			"This is not the most recent installation object",
		)
		r.Status().Update(ctx, &in)
	}
}

//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=embeddedcluster.replicated.com,resources=installations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=embeddedcluster.replicated.com,resources=installations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=embeddedcluster.replicated.com,resources=installations/finalizers,verbs=update
//+kubebuilder:rbac:groups=autopilot.k0sproject.io,resources=plans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k0s.k0sproject.io,resources=clusterconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=helm.k0sproject.io,resources=charts,verbs=get;list

// Reconcile reconcile the installation object.
func (r *InstallationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	var installs v1beta1.InstallationList
	if err := r.List(ctx, &installs); err != nil {
		return ctrl.Result{}, err
	}
	items := []v1beta1.Installation{}
	for _, in := range installs.Items {
		if in.Status.State == v1beta1.InstallationStateObsolete {
			continue
		}
		items = append(items, in)
	}
	log.Info("Reconciling installation")
	if len(items) == 0 {
		log.Info("No active installations found, reconciliation ended")
		return ctrl.Result{}, nil
	}
	in := r.CoalesceInstallations(ctx, items)
	if in.Spec.ClusterID == "" {
		log.Info("No cluster ID found, reconciliation ended")
		return ctrl.Result{}, nil
	}
	before := in.DeepCopy()
	events, err := r.ReconcileNodeStatuses(ctx, in)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile node status: %w", err)
	}
	if err := r.ReconcileK0sVersion(ctx, in); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile k0s version: %w", err)
	}
	log.Info("Reconciling addons")
	if err := r.ReconcileHelmCharts(ctx, in); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile helm charts: %w", err)
	}
	if err := r.Status().Update(ctx, in); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{}, fmt.Errorf("failed to update status: conflict")
		}
		return ctrl.Result{}, fmt.Errorf("failed to update installation status: %w", err)
	}
	r.DisableOldInstallations(ctx, items)
	if !in.Spec.AirGap {
		r.ReportInstallationChanges(ctx, before, in)
		r.ReportNodesChanges(ctx, in, events)
	}
	log.Info("Installation reconciliation ended")
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InstallationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Installation{}).
		Watches(&corev1.Node{}, &handler.EnqueueRequestForObject{}).
		Watches(&apv1b2.Plan{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
