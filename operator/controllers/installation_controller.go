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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"time"

	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/openebs"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/util"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

// requeueAfter is our default interval for requeueing. If nothing has changed with the
// cluster nodes or the Installation object we will reconcile once every requeueAfter
// interval.
var requeueAfter = time.Hour

const copyHostPreflightResultsJobPrefix = "copy-host-preflight-results-"
const ecNamespace = "embedded-cluster"

// copyHostPreflightResultsJob is a job we create everytime we need to copy host preflight results
// from a newly added node in the cluster. Host preflight are run on installation, join or restore
// operations. The results are stored in the data directory in
// /support/host-preflight-results.json. During a reconcile cycle we will populate the node
// selector, any env variables and labels.
var copyHostPreflightResultsJob = &batchv1.Job{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: ecNamespace,
	},
	Spec: batchv1.JobSpec{
		BackoffLimit:            ptr.To(int32(2)),
		TTLSecondsAfterFinished: ptr.To(int32(1 * 60)), // we don't want to keep the job around. Delete it shortly after it finishes.
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				ServiceAccountName: "embedded-cluster-operator",
				Volumes: []corev1.Volume{
					{
						Name: "host",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: ecv1beta1.DefaultDataDir,
								Type: ptr.To(corev1.HostPathDirectory),
							},
						},
					},
					{
						Name: "k0s",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: runtimeconfig.K0sBinaryPath,
								Type: ptr.To(corev1.HostPathFile),
							},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:  "copy-host-preflight-results",
						Image: "busybox:latest",
						Command: []string{
							"/bin/sh",
							"-e",
							"-c",
							"if [ -f /embedded-cluster/support/host-preflight-results.json ]; " +
								"then " +
								"/embedded-cluster/bin/kubectl create configmap ${HSPF_CM_NAME} " +
								"--from-file=results.json=/embedded-cluster/support/host-preflight-results.json " +
								"-n embedded-cluster --dry-run=client -oyaml | " +
								"/embedded-cluster/bin/kubectl label -f - embedded-cluster/host-preflight-result=${EC_NODE_NAME} --local -o yaml | " +
								"/embedded-cluster/bin/kubectl apply -f - && " +
								"/embedded-cluster/bin/kubectl annotate configmap ${HSPF_CM_NAME} \"update-timestamp=$(date +'%Y-%m-%dT%H:%M:%SZ')\" --overwrite; " +
								"else " +
								"echo '/embedded-cluster/support/host-preflight-results.json does not exist'; " +
								"fi",
						},
						Env: []corev1.EnvVar{
							{
								Name:  "KUBECONFIG",
								Value: "", // make k0s kubectl not use admin.conf
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "host",
								MountPath: "/embedded-cluster",
								ReadOnly:  false,
							},
							{
								Name:      "k0s",
								MountPath: runtimeconfig.K0sBinaryPath,
								ReadOnly:  true,
							},
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
	MetadataClient metadata.Interface
	Discovery      discovery.DiscoveryInterface
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	RuntimeConfig  runtimeconfig.RuntimeConfig
}

// NodeHasChanged returns true if the node configuration has changed when compared to
// the node information we keep in the installation status. Returns a bool indicating
// if a change was detected and a bool indicating if the node is new (not seen yet).
func (r *InstallationReconciler) NodeHasChanged(in *ecv1beta1.Installation, ev metrics.NodeEvent) (bool, bool, error) {
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
func (r *InstallationReconciler) UpdateNodeStatus(in *ecv1beta1.Installation, ev metrics.NodeEvent) error {
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
	in.Status.NodesStatus = append(in.Status.NodesStatus, ecv1beta1.NodeStatus{Name: ev.NodeName, Hash: hash})
	return nil
}

// ReconcileNodeStatuses reconciles the node statuses in the Installation object status. Installation
// is not updated remotely but only in the memory representation of the object (aka caller must save
// the object after the call). This function returns a batch of events that need to be sent back to
// the metrics endpoint, these events represent changes in the node statuses.
func (r *InstallationReconciler) ReconcileNodeStatuses(ctx context.Context, in *ecv1beta1.Installation) (*NodeEventsBatch, error) {
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	batch := &NodeEventsBatch{}
	seen := map[string]bool{}
	for _, node := range nodes.Items {
		seen[node.Name] = true
		ver := ""
		if in.Spec.Config != nil {
			ver = in.Spec.Config.Version
		}

		event := metrics.NodeEventFromNode(in.Spec.ClusterID, ver, node)
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
			r.Recorder.Eventf(in, corev1.EventTypeNormal, "NodeAdded", "Node %s has been added", node.Name)
			batch.NodesAdded = append(batch.NodesAdded, event)
			continue
		}
		r.Recorder.Eventf(in, corev1.EventTypeNormal, "NodeUpdated", "Node %s has been updated", node.Name)
		batch.NodesUpdated = append(batch.NodesUpdated, event)
	}
	trimmed := []ecv1beta1.NodeStatus{}
	for _, nodeStatus := range in.Status.NodesStatus {
		if _, ok := seen[nodeStatus.Name]; ok {
			trimmed = append(trimmed, nodeStatus)
			continue
		}
		rmevent := metrics.NodeRemovedEvent{
			ClusterID: in.Spec.ClusterID, NodeName: nodeStatus.Name,
		}
		r.Recorder.Eventf(in, corev1.EventTypeNormal, "NodeRemoved", "Node %s has been removed", nodeStatus.Name)
		batch.NodesRemoved = append(batch.NodesRemoved, rmevent)
	}
	sort.SliceStable(trimmed, func(i, j int) bool { return trimmed[i].Name < trimmed[j].Name })
	in.Status.NodesStatus = trimmed
	return batch, nil
}

// ReportNodesChanges reports node changes to the metrics endpoint.
func (r *InstallationReconciler) ReportNodesChanges(ctx context.Context, in *ecv1beta1.Installation, batch *NodeEventsBatch) {
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

func (r *InstallationReconciler) ReconcileOpenebs(ctx context.Context, in *ecv1beta1.Installation) error {
	log := ctrl.LoggerFrom(ctx)

	err := openebs.CleanupStatefulPods(ctx, r.Client)
	if err != nil {
		// Conditions may be updated so we need to update the status
		if err := r.Status().Update(ctx, in); err != nil {
			log.Error(err, "Failed to update installation status")
		}
		return fmt.Errorf("failed to cleanup openebs stateful pods: %w", err)
	}

	return nil
}

// CoalesceInstallations goes through all the installation objects and make sure that the
// status of the newest one is coherent with whole cluster status. Returns the newest
// installation object.
func (r *InstallationReconciler) CoalesceInstallations(
	ctx context.Context, items []ecv1beta1.Installation,
) *ecv1beta1.Installation {
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

// ReadClusterConfigSpecFromSecret reads the cluster config from the secret pointed by spec.ConfigSecret
// if it is set. This overrides the default configuration from spec.Config.
func (r *InstallationReconciler) ReadClusterConfigSpecFromSecret(ctx context.Context, in *ecv1beta1.Installation) error {
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

// CopyHostPreflightResultsFromNodes copies the preflight results from any new node that is added to the cluster
// A job is scheduled on the new node and the results copied from a host path
func (r *InstallationReconciler) CopyHostPreflightResultsFromNodes(ctx context.Context, in *ecv1beta1.Installation, events *NodeEventsBatch) error {
	log := ctrl.LoggerFrom(ctx)

	if len(events.NodesAdded) == 0 {
		log.Info("No new nodes added to the cluster, skipping host preflight results copy job creation")
		return nil
	}

	for _, event := range events.NodesAdded {
		log.Info("Creating job to copy host preflight results from node", "node", event.NodeName, "installation", in.Name)

		job := constructHostPreflightResultsJob(r.RuntimeConfig, in, event.NodeName)

		// overrides the job image if the environment says so.
		if img := os.Getenv("EMBEDDEDCLUSTER_UTILS_IMAGE"); img != "" {
			job.Spec.Template.Spec.Containers[0].Image = img
		}

		if err := r.Create(ctx, job); err != nil {
			if !k8serrors.IsAlreadyExists(err) {
				return fmt.Errorf("failed to create job: %w", err)
			}
		} else {
			log.Info("Copy host preflight results job for node created", "node", event.NodeName, "installation", in.Name)
		}
	}

	return nil
}

func constructHostPreflightResultsJob(rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation, nodeName string) *batchv1.Job {
	labels := map[string]string{
		"embedded-cluster/node-name":    nodeName,
		"embedded-cluster/installation": in.Name,
	}

	job := copyHostPreflightResultsJob.DeepCopy()
	job.Name = util.NameWithLengthLimit(copyHostPreflightResultsJobPrefix, nodeName)

	job.Spec.Template.Labels, job.Labels = labels, labels
	job.Spec.Template.Spec.NodeName = nodeName
	job.Spec.Template.Spec.Volumes[0].VolumeSource.HostPath.Path = rc.EmbeddedClusterHomeDirectory()
	job.Spec.Template.Spec.Containers[0].Env = append(
		job.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "EC_NODE_NAME", Value: nodeName},
		corev1.EnvVar{Name: "HSPF_CM_NAME", Value: util.NameWithLengthLimit(nodeName, "-host-preflight-results")},
	)

	return job
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
	installs, err := kubeutils.ListInstallations(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list installations: %w", err)
	}
	var items []ecv1beta1.Installation
	for _, in := range installs {
		if in.Status.State == ecv1beta1.InstallationStateObsolete {
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

	// set the runtime config from the installation spec
	r.RuntimeConfig.Set(in.Spec.RuntimeConfig)

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
		return ctrl.Result{}, fmt.Errorf("failed to read cluster config from secret: %w", err)
	}

	// verify if a new node has been added, removed or changed.
	events, err := r.ReconcileNodeStatuses(ctx, in)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile node status: %w", err)
	}

	// Copy host preflight results to a configmap for each node
	if err := r.CopyHostPreflightResultsFromNodes(ctx, in, events); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to copy host preflight results: %w", err)
	}

	// cleanup openebs stateful pods
	if err := r.ReconcileOpenebs(ctx, in); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile openebs: %w", err)
	}

	// save the installation status. nothing more to do with it.
	if err := r.Status().Update(ctx, in); err != nil {
		if k8serrors.IsConflict(err) {
			return ctrl.Result{}, fmt.Errorf("failed to update status: conflict")
		}
		return ctrl.Result{}, fmt.Errorf("failed to update installation status: %w", err)
	}

	// if we are not in an airgap environment this is the time to call back to
	// replicated and inform the status of this installation.
	if !in.Spec.AirGap {
		r.ReportNodesChanges(ctx, in, events)
	}

	// ensure the CA configmap is present and up-to-date
	if err := r.reconcileHostCABundle(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure kotsadm CA configmap: %w", err)
	}

	log.Info("Installation reconciliation ended")
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// reconcileHostCABundle ensures that the CA configmap is present and is up-to-date
// with the CA bundle from the host.
func (r *InstallationReconciler) reconcileHostCABundle(ctx context.Context) error {
	caPathInContainer := os.Getenv("PRIVATE_CA_BUNDLE_PATH")
	if caPathInContainer == "" {
		return nil
	}

	err := r.Get(ctx, types.NamespacedName{Name: "kotsadm"}, &corev1.Namespace{})
	if k8serrors.IsNotFound(err) {
		// if the namespace has not been created yet, we don't need to reconcile the CA configmap
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get kotsadm namespace: %w", err)
	}

	logger := ctrl.LoggerFrom(ctx)
	logf := func(format string, args ...interface{}) {
		logger.Info(fmt.Sprintf(format, args...))
	}
	err = adminconsole.EnsureCAConfigmap(ctx, logf, r.Client, r.MetadataClient, caPathInContainer)
	if k8serrors.IsRequestEntityTooLargeError(err) || errors.Is(err, fs.ErrNotExist) {
		logger.Error(err, "Failed to reconcile host ca bundle")
		return nil
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *InstallationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ecv1beta1.Installation{}).
		Watches(&corev1.Node{}, &handler.EnqueueRequestForObject{}).
		Watches(&apv1b2.Plan{}, &handler.EnqueueRequestForObject{}).
		Watches(&k0shelm.Chart{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
