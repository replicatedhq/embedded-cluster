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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apcore "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	"github.com/k0sproject/version"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/registry"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/artifacts"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/autopilot"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/release"
)

// InstallationNameAnnotation is the annotation we keep in the autopilot plan so we can
// map 1 to 1 one installation and one plan.
const InstallationNameAnnotation = "embedded-cluster.replicated.com/installation-name"

// requeueAfter is our default interval for requeueing. If nothing has changed with the
// cluster nodes or the Installation object we will reconcile once every requeueAfter
// interval.
var requeueAfter = time.Hour

const copyArtifactsJobPrefix = "copy-artifacts-"

// copyArtifactsJob is a job we create everytime we need to sync files into all nodes.
// This job mounts /var/lib/embedded-cluster from the node and uses binaries that are
// present there. This is not yet a complete version of the job as it misses some env
// variables and a node selector, those are populated during the reconcile cycle.
var copyArtifactsJob = &batchv1.Job{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "embedded-cluster",
	},
	Spec: batchv1.JobSpec{
		BackoffLimit: ptr.To[int32](2),
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				ServiceAccountName: "embedded-cluster-operator",
				Volumes: []corev1.Volume{
					{
						Name: "host",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/var/lib/embedded-cluster",
								Type: ptr.To[corev1.HostPathType]("Directory"),
							},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:  "embedded-cluster-updater",
						Image: "busybox:latest",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "host",
								MountPath: "/var/lib/embedded-cluster",
								ReadOnly:  false,
							},
						},
						Command: []string{
							"/bin/sh",
							"-e",
							"-c",
							"/var/lib/embedded-cluster/bin/local-artifact-mirror pull binaries $INSTALLATION\n" +
								"/var/lib/embedded-cluster/bin/local-artifact-mirror pull images $INSTALLATION\n" +
								"/var/lib/embedded-cluster/bin/local-artifact-mirror pull helmcharts $INSTALLATION\n" +
								"mv /var/lib/embedded-cluster/bin/k0s /var/lib/embedded-cluster/bin/k0s-upgrade\n" +
								"rm /var/lib/embedded-cluster/images/images-amd64-* || true\n" +
								"cd /var/lib/embedded-cluster/images/\n" +
								"mv images-amd64.tar images-amd64-${INSTALLATION}.tar\n" +
								"echo 'done'",
						},
					},
				},
			},
		},
	},
}

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

// HashForAirgapConfig generates a hash for the airgap configuration. We can use this to detect config changes between
// different reconcile cycles.
func (r *InstallationReconciler) HashForAirgapConfig(in *v1beta1.Installation) (string, error) {
	data, err := json.Marshal(in.Spec.Artifacts)
	if err != nil {
		return "", fmt.Errorf("failed to marshal artifacts location: %w", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	return hash[:10], nil
}

// CreateArtifactJobForNode creates a job to copy assets from the internal registry. This jobs runs in the provided node.
func (r *InstallationReconciler) CreateArtifactJobForNode(ctx context.Context, in *v1beta1.Installation, node corev1.Node) error {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Creating artifact job for node", "node", node.Name, "installation", in.Name)

	hash, err := r.HashForAirgapConfig(in)
	if err != nil {
		return fmt.Errorf("failed to hash airgap config: %w", err)
	}

	labels := map[string]string{
		"embedded-cluster/installation":          in.Name,
		"embedded-cluster/artifacts-config-hash": hash,
	}
	job := copyArtifactsJob.DeepCopy()
	job.Name = util.NameWithLengthLimit(copyArtifactsJobPrefix, node.Name)

	job.Spec.Template.Labels, job.Labels = labels, labels
	job.Spec.Template.Spec.NodeName = node.Name
	job.Spec.Template.Spec.Containers[0].Env = append(
		job.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "INSTALLATION", Value: in.Name},
	)

	// overrides the job image if the environment says so.
	if img := os.Getenv("EMBEDDEDCLUSTER_UTILS_IMAGE"); img != "" {
		job.Spec.Template.Spec.Containers[0].Image = img
	}

	if err := r.Create(ctx, job); err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	log.Info("Artifact job for node created", "node", node.Name, "installation", in.Name)
	return nil
}

// CopyVersionMetadataToCluster makes sure a config map with the embedded cluster version metadata exists in the
// cluster. The data is read from the internal registry on the repository pointed by EmbeddedClusterMetadata.
func (r *InstallationReconciler) CopyVersionMetadataToCluster(ctx context.Context, in *v1beta1.Installation) error {
	log := ctrl.LoggerFrom(ctx)

	// if there is no configuration, no version inside the configuration or the no artifacts location
	// we log and skip as we can't determine for which version nor from where to retrieve the version
	// metadata.
	if in.Spec.Artifacts == nil || in.Spec.Config == nil || in.Spec.Config.Version == "" {
		log.Info("Skipping version metadata copy to cluster", "installation", in.Name)
		return nil
	}

	// let's first verify if we haven't yet fetched the metadata for the specified version. if we found
	// the config map then it means we have already copied the data to the cluster and we can move on.
	nsn := release.LocalVersionMetadataConfigmap(in.Spec.Config.Version)
	var cm corev1.ConfigMap
	if err := r.Get(ctx, nsn, &cm); err == nil {
		return nil
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get version metadata configmap: %w", err)
	}

	// pull the artifact from the artifact location pointed by EmbeddedClusterMetadata. This property
	// points to a repository inside the registry running on the cluster.
	location, err := artifacts.Pull(ctx, log, r.Client, in.Spec.Artifacts.EmbeddedClusterMetadata)
	if err != nil {
		return fmt.Errorf("failed to pull version metadata: %w", err)
	}
	defer os.RemoveAll(location)

	// now that we have the metadata locally we can reads its information and create the config map.
	fpath := filepath.Join(location, "version-metadata.json")
	data, err := os.ReadFile(fpath)
	if err != nil {
		return fmt.Errorf("failed to read version metadata: %w", err)
	}

	cm.Name = nsn.Name
	cm.Namespace = nsn.Namespace
	cm.Data = map[string]string{"metadata.json": string(data)}
	if err := r.Create(ctx, &cm); err != nil {
		return fmt.Errorf("failed to create version metadata configmap: %w", err)
	}
	return nil
}

// CopyArtifactsToNodes copies the installation artifacts to the nodes in the cluster.
// This is done by creating a job for each node in the cluster, which will pull the
// artifacts from the internal registry.
func (r *InstallationReconciler) CopyArtifactsToNodes(ctx context.Context, in *v1beta1.Installation) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Evaluating jobs for copying artifacts to nodes", "installation", in.Name)
	if in.Spec.Artifacts == nil {
		in.Status.State = v1beta1.InstallationStateFailed
		in.Status.Reason = "Artifacts locations not specified for an airgap installation"
		return false, nil
	}

	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		return false, fmt.Errorf("failed to list nodes: %w", err)
	}

	// generate a hash of the current config so we can detect config changes.
	cfghash, err := r.HashForAirgapConfig(in)
	if err != nil {
		return false, fmt.Errorf("failed to hash airgap config: %w", err)
	}

	status := map[string]string{}
	ready := true
	for _, node := range nodes.Items {
		log.Info("Evaluating job for node", "node", node.Name)
		nsn := types.NamespacedName{
			Name:      util.NameWithLengthLimit(copyArtifactsJobPrefix, node.Name),
			Namespace: "embedded-cluster",
		}

		// we first verify if a job already exists for the node, if not then one is
		// created and we move to the next node.
		var job batchv1.Job
		if err := r.Get(ctx, nsn, &job); err != nil {
			if !errors.IsNotFound(err) {
				return false, fmt.Errorf("failed to get job: %w", err)
			}
			ready = false
			status[node.Name] = "JobCreated"
			log.Info("No job for node found", "node", node.Name)
			if err := r.CreateArtifactJobForNode(ctx, in, node); err != nil {
				return false, fmt.Errorf("failed to create job for node: %w", err)
			}
			continue
		}

		// we need to check if the job is for the given installation otherwise we delete
		// it. we also need to check if the configuration has changed. this will trigger
		// a new reconcile cycle.
		oldjob := job.Labels["embedded-cluster/installation"] != in.Name
		newcfg := job.Labels["embedded-cluster/artifacts-config-hash"] != cfghash
		if oldjob || newcfg {
			log.Info("Deleting previous job", "oldJob", oldjob, "configchange", newcfg)
			ready = false
			status[node.Name] = "WaitingPreviousJobDeletion"
			policy := metav1.DeletePropagationForeground
			opt := &client.DeleteOptions{PropagationPolicy: &policy}
			if err := r.Delete(ctx, &job, opt); err != nil {
				return false, fmt.Errorf("failed to delete old job: %w", err)
			}
			continue
		}

		// from now on we know we analysing the correct job for the installation.
		if job.Status.Succeeded > 0 {
			status[node.Name] = "JobSucceeded"
			log.Info("Job for node succeeded", "node", node.Name)
			continue
		}

		ready = false
		status[node.Name] = "JobRunning"
		for _, cond := range job.Status.Conditions {
			if cond.Type != batchv1.JobFailed {
				continue
			}
			if cond.Status != corev1.ConditionTrue {
				continue
			}
			log.Info("Job for node found in a faulty state", "node", node.Name)
			status[node.Name] = fmt.Sprintf("JobFailed: %s", cond.Message)
		}
		log.Info("Job for node still running", "node", node.Name)
	}

	if ready {
		return true, nil
	}

	all := []string{}
	for name, state := range status {
		all = append(all, fmt.Sprintf("%s(%s)", name, state))
	}
	in.Status.Reason = fmt.Sprintf("Copying artifacts to nodes: %s", strings.Join(all, ", "))
	in.Status.State = v1beta1.InstallationStateCopyingArtifacts
	if strings.Contains(in.Status.Reason, "JobFailed") {
		in.Status.State = v1beta1.InstallationStateFailed
	}
	return false, nil
}

// HasOnlyOneInstallation returns true if only one Installation object exists in the cluster.
func (r *InstallationReconciler) HasOnlyOneInstallation(ctx context.Context) (bool, error) {
	var list v1beta1.InstallationList
	if err := r.List(ctx, &list); err != nil {
		return false, fmt.Errorf("failed to list installations: %w", err)
	}
	return len(list.Items) == 1, nil
}

// ReconcileK0sVersion reconciles the k0s version in the Installation object status. If the
// Installation spec.config points to a different version we start an upgrade Plan. If an
// upgrade plan already exists we make sure the installation status is updated with the
// latest plan status.
func (r *InstallationReconciler) ReconcileK0sVersion(ctx context.Context, in *v1beta1.Installation) error {
	// starts by checking if this is the unique installation object in the cluster. if
	// this is true then we don't need to sync anything as this is part of the initial
	// cluster installation.
	uniqinst, err := r.HasOnlyOneInstallation(ctx)
	if err != nil {
		return fmt.Errorf("failed to find if there are multiple installations: %w", err)
	}

	// if the installation has no desired version then there isn't much we can do other
	// than flagging as installed. if there is also only one installation object in the
	// cluster then there is no upgrade to be executed, just set it to Installed and
	// move on.
	if in.Spec.Config == nil || in.Spec.Config.Version == "" || uniqinst {
		in.Status.SetState(v1beta1.InstallationStateKubernetesInstalled, "", nil)
		return nil
	}

	// in airgap installation the first thing we need to do is to ensure that the embedded
	// cluster version metadata is available inside the cluster. we can't use the internet
	// to fetch it directly from our remote servers.
	if in.Spec.AirGap {
		if err := r.CopyVersionMetadataToCluster(ctx, in); err != nil {
			return fmt.Errorf("failed to copy version metadata to cluster: %w", err)
		}
	}

	// fetch the metadata for the desired embedded cluster version.
	meta, err := release.MetadataFor(ctx, in, r.Client)
	if err != nil {
		in.Status.SetState(v1beta1.InstallationStateFailed, err.Error(), nil)
		return nil
	}

	// find out the kubernetes version we are currently running so we can compare with
	// the desired kubernetes version. we don't want anyone trying to do a downgrade.
	vinfo, err := r.Discovery.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get server version: %w", err)
	}
	runningVersion := vinfo.GitVersion
	running, err := version.NewVersion(runningVersion)
	if err != nil {
		reason := fmt.Sprintf("Invalid running version %s", runningVersion)
		in.Status.SetState(v1beta1.InstallationStateFailed, reason, nil)
		return nil
	}

	// if we have installed the cluster with a k0s version like v1.29.1+k0s.1 then
	// the kubernetes server version reported back is v1.29.1+k0s. i.e. the .1 is
	// not part of the kubernetes version, it is the k0s version. we trim it down
	// so we can compare kube with kube version.
	desiredVersion := meta.Versions["Kubernetes"]
	index := strings.Index(desiredVersion, "k0s")
	if index == -1 {
		reason := fmt.Sprintf("Invalid desired version %s", desiredVersion)
		in.Status.SetState(v1beta1.InstallationStateFailed, reason, nil)
		return nil
	}
	desiredVersion = desiredVersion[:index+len("k0s")]
	desired, err := version.NewVersion(desiredVersion)
	if err != nil {
		reason := fmt.Sprintf("Invalid desired version %s", in.Spec.Config.Version)
		in.Status.SetState(v1beta1.InstallationStateFailed, reason, nil)
		return nil
	}

	// stop here if someone is trying a downgrade. we do not support this, flag the
	// installation accordingly and returns.
	if running.GreaterThan(desired) {
		in.Status.SetState(v1beta1.InstallationStateFailed, "Downgrades not supported", nil)
		return nil
	}

	if in.Spec.AirGap {
		// in airgap installations let's make sure all assets have been copied to nodes.
		// this may take some time so we only move forward when 'ready'.
		if ready, err := r.CopyArtifactsToNodes(ctx, in); err != nil {
			return fmt.Errorf("failed to copy artifacts to nodes: %w", err)
		} else if !ready {
			return nil
		}
	}

	var plan apv1b2.Plan
	okey := client.ObjectKey{Name: "autopilot"}
	if err := r.Get(ctx, okey, &plan); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get upgrade plan: %w", err)
	} else if errors.IsNotFound(err) {
		// there is no autopilot plan in the cluster so we are free to
		// start our own plan. here we link the plan to the installation
		// by its name.
		if err := r.StartUpgrade(ctx, in); err != nil {
			return fmt.Errorf("failed to start upgrade: %w", err)
		}
		return nil
	}

	// if we have created this plan we just found for the installation we are
	// reconciling we set the installation state according to the plan state.
	// we check both the plan id and an annotation inside the plan. the usage
	// of the plan id is deprecated in favour of the annotation.
	annotation := plan.Annotations[InstallationNameAnnotation]
	if annotation == in.Name || plan.Spec.ID == in.Name {
		r.SetStateBasedOnPlan(in, plan)
		return nil
	}

	// this is most likely a plan that has been created by a previous installation
	// object, we can't move on until this one finishes. this can happen if someone
	// issues multiple upgrade requests at the same time.
	if !autopilot.HasThePlanEnded(plan) {
		reason := fmt.Sprintf("Another upgrade is in progress (%s)", plan.Spec.ID)
		in.Status.SetState(v1beta1.InstallationStateWaiting, reason, nil)
		return nil
	}

	// it seems like the plan previously created by other installation object
	// has been finished, we can delete it. this will trigger a new reconcile
	// this time without the plan (i.e. we will be able to create our own plan).
	if err := r.Delete(ctx, &plan); err != nil {
		return fmt.Errorf("failed to delete previous upgrade plan: %w", err)
	}
	return nil
}

// ReconcileRegistry reconciles registry components, ensuring that the necessary secrets are
// created as well as rebalancing stateful pods when nodes are removed from the cluster.
func (r *InstallationReconciler) ReconcileRegistry(ctx context.Context, in *v1beta1.Installation) error {
	if in == nil || !in.Spec.AirGap || !in.Spec.HighAvailability {
		return nil
	}

	log := ctrl.LoggerFrom(ctx)

	// fetch the current clusterConfig
	var clusterConfig k0sv1beta1.ClusterConfig
	if err := r.Get(ctx, client.ObjectKey{Name: "k0s", Namespace: "kube-system"}, &clusterConfig); err != nil {
		return fmt.Errorf("failed to get cluster config: %w", err)
	}

	serviceCIDR := k0sv1beta1.DefaultNetwork().ServiceCIDR
	if clusterConfig.Spec != nil && clusterConfig.Spec.Network != nil {
		serviceCIDR = clusterConfig.Spec.Network.ServiceCIDR
	}

	err := registry.EnsureResources(ctx, in, r.Client, serviceCIDR)
	if err != nil {
		// Conditions may be updated so we need to update the status
		if err := r.Status().Update(ctx, in); err != nil {
			log.Error(err, "Failed to update installation status")
		}
		return fmt.Errorf("failed to ensure registry resources: %w", err)
	}

	return nil
}

// ReconcileHelmCharts reconciles the helm charts from the Installation metadata with the clusterconfig object.
func (r *InstallationReconciler) ReconcileHelmCharts(ctx context.Context, in *v1beta1.Installation) error {
	if in.Spec.Config == nil || in.Spec.Config.Version == "" {
		if in.Status.State == v1beta1.InstallationStateKubernetesInstalled {
			in.Status.SetState(v1beta1.InstallationStateInstalled, "Installed", nil)
		}
		return nil
	}

	log := ctrl.LoggerFrom(ctx)
	// skip if the installer has already completed, failed or if the k0s upgrade is still in progress
	if in.Status.State == v1beta1.InstallationStateFailed ||
		in.Status.State == v1beta1.InstallationStateInstalled ||
		!in.Status.GetKubernetesInstalled() {
		log.Info("Skipping helm chart reconciliation", "state", in.Status.State)
		return nil
	}

	meta, err := release.MetadataFor(ctx, in, r.Client)
	if err != nil {
		in.Status.SetState(v1beta1.InstallationStateHelmChartUpdateFailure, err.Error(), nil)
		return nil
	}

	// skip if the new release has no addon configs - this should not happen in production
	if len(meta.Configs.Charts) == 0 {
		log.Info("Addons", "configcheck", "no addons")
		if in.Status.State == v1beta1.InstallationStateKubernetesInstalled {
			in.Status.SetState(v1beta1.InstallationStateInstalled, "Installed", nil)
		}
		return nil
	}

	combinedConfigs := mergeHelmConfigs(ctx, meta, in)

	// fetch the current clusterConfig
	var clusterConfig k0sv1beta1.ClusterConfig
	if err := r.Get(ctx, client.ObjectKey{Name: "k0s", Namespace: "kube-system"}, &clusterConfig); err != nil {
		return fmt.Errorf("failed to get cluster config: %w", err)
	}

	if in.Spec.AirGap {
		// if in airgap mode then all charts are already on the node's disk. we just need to
		// make sure that the helm charts are pointing to the right location on disk and that
		// we do not have any kind of helm repository configuration.
		combinedConfigs = patchExtensionsForAirGap(combinedConfigs)
	}

	combinedConfigs, err = applyUserProvidedAddonOverrides(in, combinedConfigs)
	if err != nil {
		return fmt.Errorf("failed to apply user provided overrides: %w:", err)
	}

	existingHelm := &k0sv1beta1.HelmExtensions{}
	if clusterConfig.Spec != nil && clusterConfig.Spec.Extensions != nil && clusterConfig.Spec.Extensions.Helm != nil {
		existingHelm = clusterConfig.Spec.Extensions.Helm
	}

	chartDrift, changedCharts, err := detectChartDrift(combinedConfigs, existingHelm)
	if err != nil {
		return fmt.Errorf("failed to check chart drift: %w", err)
	}

	// detect drift between the cluster config and the installer metadata
	var installedCharts k0shelm.ChartList
	if err := r.List(ctx, &installedCharts); err != nil {
		return fmt.Errorf("failed to list installed charts: %w", err)
	}
	pendingCharts, chartErrors, err := detectChartCompletion(existingHelm, installedCharts)
	if err != nil {
		return fmt.Errorf("failed to check chart completion: %w", err)
	}

	// If any chart has errors, update installer state and return
	// if there is a difference between what we want and what we have
	// we should update the cluster instead of letting chart errors stop deployment permanently
	if len(chartErrors) > 0 && !chartDrift {
		chartErrorString := strings.Join(chartErrors, ",")
		chartErrorString = "failed to update helm charts: " + chartErrorString
		log.Info("Chart errors", "errors", chartErrorString)
		if len(chartErrorString) > 1024 {
			chartErrorString = chartErrorString[:1024]
		}
		in.Status.SetState(v1beta1.InstallationStateHelmChartUpdateFailure, chartErrorString, nil)
		return nil
	}

	// If all addons match their target version + values, mark installation as complete
	if len(pendingCharts) == 0 && !chartDrift {
		in.Status.SetState(v1beta1.InstallationStateInstalled, "Addons upgraded", nil)
		return nil
	}

	if len(pendingCharts) > 0 {
		// If there are pending charts, mark the installation as pending with a message about the pending charts
		in.Status.SetState(v1beta1.InstallationStatePendingChartCreation, fmt.Sprintf("Pending charts: %v", pendingCharts), pendingCharts)
		return nil
	}

	if in.Status.State == v1beta1.InstallationStateAddonsInstalling {
		// after the first time we apply new helm charts, this will be set to InstallationStateAddonsInstalling
		// and we will not re-apply the charts to the k0s cluster config while waiting for those changes to propagate
		return nil
	}

	if !chartDrift {
		// if there is no drift, we should not reapply the cluster config
		// however, the charts have not been applied yet, so we should not mark the installation as complete
		return nil
	}

	// Replace the current chart configs with the new chart configs
	clusterConfig.Spec.Extensions.Helm = combinedConfigs
	in.Status.SetState(v1beta1.InstallationStateAddonsInstalling, "Installing addons", nil)
	log.Info("Updating cluster config with new helm charts", "updated charts", changedCharts)
	//Update the clusterConfig
	if err := r.Update(ctx, &clusterConfig); err != nil {
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
		in.Status.SetState(v1beta1.InstallationStateEnqueued, reason, nil)
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
		in.Status.SetState(v1beta1.InstallationStateFailed, reason, nil)
	case apcore.PlanSchedulable:
		fallthrough
	case apcore.PlanSchedulableWait:
		in.Status.SetState(v1beta1.InstallationStateInstalling, reason, nil)
	case apcore.PlanCompleted:
		in.Status.SetState(v1beta1.InstallationStateKubernetesInstalled, reason, nil)
	default:
		in.Status.SetState(v1beta1.InstallationStateFailed, reason, nil)
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

// CreateAirgapPlanCommand creates the plan to execute an aigrap upgrade in all nodes. The
// return of this function is meant to be used as part of an autopilot plan.
func (r *InstallationReconciler) CreateAirgapPlanCommand(ctx context.Context, in *v1beta1.Installation) (*apv1b2.PlanCommand, error) {
	meta, err := release.MetadataFor(ctx, in, r.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to get release bundle: %w", err)
	}

	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var allNodes []string
	for _, node := range nodes.Items {
		allNodes = append(allNodes, node.Name)
	}

	imageURL := fmt.Sprintf("http://127.0.0.1:50000/images/images-amd64-%s.tar", in.Name)

	return &apv1b2.PlanCommand{
		AirgapUpdate: &apv1b2.PlanCommandAirgapUpdate{
			Version: meta.Versions["Kubernetes"],
			Platforms: map[string]apv1b2.PlanResourceURL{
				"linux-amd64": {
					URL: imageURL,
				},
			},
			Workers: apv1b2.PlanCommandTarget{
				Discovery: apv1b2.PlanCommandTargetDiscovery{
					Static: &apv1b2.PlanCommandTargetDiscoveryStatic{
						Nodes: allNodes,
					},
				},
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
	meta, err := release.MetadataFor(ctx, in, r.Client)
	if err != nil {
		return fmt.Errorf("failed to get release bundle: %w", err)
	}

	k0surl := fmt.Sprintf(
		"%s/embedded-cluster-public-files/k0s-binaries/%s",
		in.Spec.MetricsBaseURL,
		meta.Versions["Kubernetes"],
	)

	// we need to assess what commands should autopilot run upon this upgrade. we can have four
	// different scenarios: 1) we are upgrading only the airgap artifacts, 2) we are upgrading
	// only k0s binaries, 3) we are upgrading both, 4) we are upgrading neither. we populate the
	// 'commands' slice with the commands necessary to execute these operations.
	var commands []apv1b2.PlanCommand

	if in.Spec.AirGap {
		// if we are running in an airgap environment all assets are already present in the
		// node and are served by the local-artifact-mirror binary listening on localhost
		// port 50000. we just need to get autopilot to fetch the k0s binary from there.
		k0surl = "http://127.0.0.1:50000/bin/k0s-upgrade"
		command, err := r.CreateAirgapPlanCommand(ctx, in)
		if err != nil {
			return fmt.Errorf("failed to create airgap plan command: %w", err)
		}
		commands = append(commands, *command)
	}

	// if the embedded cluster version has changed we create an upgrade command.
	curstr := strings.TrimPrefix(os.Getenv("EMBEDDEDCLUSTER_VERSION"), "v")
	desstr := strings.TrimPrefix(in.Spec.Config.Version, "v")
	if curstr != desstr {
		commands = append(commands, apv1b2.PlanCommand{
			K0sUpdate: &apv1b2.PlanCommandK0sUpdate{
				Version: meta.Versions["Kubernetes"],
				Targets: targets,
				Platforms: apv1b2.PlanPlatformResourceURLMap{
					"linux-amd64": {URL: k0surl, Sha256: meta.K0sSHA},
				},
			},
		})
	}

	// if no airgap nor k0s upgrade has been defined it means we are up to date so we set
	// the installation state to 'Installed' and return. no extra autopilot plan creation
	// is necessary at this stage.
	if len(commands) == 0 {
		in.Status.SetState(v1beta1.InstallationStateKubernetesInstalled, "", nil)
		return nil
	}

	plan := apv1b2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
			Annotations: map[string]string{
				InstallationNameAnnotation: in.Name,
			},
		},
		Spec: apv1b2.PlanSpec{
			Timestamp: "now",
			ID:        uuid.New().String(),
			Commands:  commands,
		},
	}
	if err := r.Create(ctx, &plan); err != nil {
		return fmt.Errorf("failed to create upgrade plan: %w", err)
	}
	in.Status.SetState(v1beta1.InstallationStateEnqueued, "", nil)
	return nil
}

// CoalesceInstallations goes through all the installation objects and make sure that the
// status of the newest one is coherent with whole cluster status. Returns the newest
// installation object.
func (r *InstallationReconciler) CoalesceInstallations(
	ctx context.Context, items []v1beta1.Installation,
) *v1beta1.Installation {
	sort.SliceStable(items, func(i, j int) bool {
		return items[j].Name < items[i].Name
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
		return items[j].Name < items[i].Name
	})
	for _, in := range items[1:] {
		in.Status.NodesStatus = nil
		in.Status.SetState(
			v1beta1.InstallationStateObsolete,
			"This is not the most recent installation object",
			nil,
		)
		r.Status().Update(ctx, &in)
	}
}

// ReadClusterConfigSpecFromSecret reads the cluster config from the secret pointed by spec.ConfigSecret
// if it is set. This overrides the default configuration from spec.Config.
func (r *InstallationReconciler) ReadClusterConfigSpecFromSecret(ctx context.Context, in *v1beta1.Installation) error {
	if in.Spec.ConfigSecret == nil {
		return nil
	}
	var secret corev1.Secret
	nsn := types.NamespacedName{Namespace: in.Spec.ConfigSecret.Namespace, Name: in.Spec.ConfigSecret.Name}
	if err := r.Get(ctx, nsn, &secret); err != nil {
		return fmt.Errorf("failed to get config secret: %w", err)
	}
	if err := in.Spec.ParseConfigSpecFromSecret(secret); err != nil {
		return fmt.Errorf("failed to parse config spec from secret: %w", err)
	}
	return nil
}

//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=embeddedcluster.replicated.com,resources=installations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=embeddedcluster.replicated.com,resources=installations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=embeddedcluster.replicated.com,resources=installations/finalizers,verbs=update
//+kubebuilder:rbac:groups=autopilot.k0sproject.io,resources=plans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k0s.k0sproject.io,resources=clusterconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=helm.k0sproject.io,resources=charts,verbs=get;list;watch

// Reconcile reconcile the installation object.
func (r *InstallationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// we start by fetching all installation objects and coalescing them. we
	// are going to operate only on the newest one (sorting by installation
	// name).
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

	// if this cluster has no id we bail out immediately.
	if in.Spec.ClusterID == "" {
		log.Info("No cluster ID found, reconciliation ended")
		return ctrl.Result{}, nil
	}

	// if this installation points to a cluster configuration living on
	// a secret we need to fetch this configuration before moving on.
	// at this stage we bail out with an error if we can't fetch or
	// parse the config otherwise we risk moving on with a reconcile
	// using an erroneous config.
	if err := r.ReadClusterConfigSpecFromSecret(ctx, in); err != nil {
		in.Status.SetState(v1beta1.InstallationStateFailed, err.Error(), nil)
		if err := r.Status().Update(ctx, in); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update installation status: %w", err)
		}
		r.DisableOldInstallations(ctx, items)
		return ctrl.Result{}, fmt.Errorf("failed to update installation status: %w", err)
	}

	// we create a copy of the installation so we can compare if it
	// changed its status after the reconcile (this is mostly for
	// calling back to us with events).
	before := in.DeepCopy()

	// verify if a new node has been added, removed or changed.
	events, err := r.ReconcileNodeStatuses(ctx, in)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile node status: %w", err)
	}

	// if necessary start a k0s upgrade by means of autopilot. this also
	// keeps the installation in sync with the state of the k0s upgrade.
	if err := r.ReconcileK0sVersion(ctx, in); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile k0s version: %w", err)
	}

	// reconcile helm chart dependencies including secrets.
	if err := r.ReconcileRegistry(ctx, in); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to pre-reconcile helm charts: %w", err)
	}

	// reconcile the add-ons (k0s helm extensions).
	log.Info("Reconciling addons")
	if err := r.ReconcileHelmCharts(ctx, in); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile helm charts: %w", err)
	}

	// save the installation status. nothing more to do with it.
	if err := r.Status().Update(ctx, in.DeepCopy()); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{}, fmt.Errorf("failed to update status: conflict")
		}
		return ctrl.Result{}, fmt.Errorf("failed to update installation status: %w", err)
	}

	// now that the status has been updated we can flag all older installation
	// objects as obsolete. these are not necessary anymore and are kept only
	// for historic reasons.
	r.DisableOldInstallations(ctx, items)

	// if we are not in an airgap environment this is the time to call back to
	// replicated and inform the status of this installation.
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
		Owns(&corev1.Secret{}).
		Owns(&corev1.Service{}).
		Watches(&corev1.Node{}, &handler.EnqueueRequestForObject{}).
		Watches(&apv1b2.Plan{}, &handler.EnqueueRequestForObject{}).
		Watches(&k0shelm.Chart{}, &handler.EnqueueRequestForObject{}).
		Watches(&batchv1.Job{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
