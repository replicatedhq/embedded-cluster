package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/utils/ptr"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/artifacts"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/autopilot"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/charts"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/metadata"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	operatorChartName      = "embedded-cluster-operator"
	clusterConfigName      = "k0s"
	clusterConfigNamespace = "kube-system"
	upgradeJobName         = "embedded-cluster-upgrade-%s"
	upgradeJobConfigMap    = "upgrade-job-configmap-%s"
)

// CreateUpgradeJob creates a job that upgrades the embedded cluster to the version specified in the installation.
// if the installation is airgapped, the artifacts are copied to the nodes and the autopilot plan is
// created to copy the images to the cluster. A comfigmap is then created containing the target installation
// spec and the upgrade job is created. The upgrade job will update the cluster version, and then update the operator chart.
func CreateUpgradeJob(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation, localArtifactMirrorImage string) error {
	// check if the job already exists - if it does, we've already rolled out images and can return now
	job := &batchv1.Job{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: "embedded-cluster", Name: fmt.Sprintf(upgradeJobName, in.Name)}, job)
	if err == nil {
		return nil
	}

	if in.Spec.AirGap {
		// in airgap installations we need to copy the artifacts to the nodes and then autopilot
		// will copy the images to the cluster so we can start the new operator.

		if localArtifactMirrorImage == "" {
			return fmt.Errorf("local artifact mirror image is required for airgap installations")
		}

		err = metadata.CopyVersionMetadataToCluster(ctx, cli, in)
		if err != nil {
			return fmt.Errorf("copy version metadata to cluster: %w", err)
		}

		err = airgapDistributeArtifacts(ctx, cli, in, localArtifactMirrorImage)
		if err != nil {
			return fmt.Errorf("airgap distribute artifacts: %w", err)
		}
	}

	err = createInstallation(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("apply installation: %w", err)
	}

	// create the upgrade job configmap with the target installation spec
	installationData, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("failed to marshal installation spec: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "embedded-cluster",
			Name:      fmt.Sprintf(upgradeJobConfigMap, in.Name),
		},
		Data: map[string]string{
			"installation.yaml": string(installationData),
		},
	}
	if err = cli.Create(ctx, cm); err != nil {
		return fmt.Errorf("failed to create upgrade job configmap: %w", err)
	}

	operatorImage, err := operatorImageName(ctx, cli, in)
	if err != nil {
		return err
	}

	// create the upgrade job
	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "embedded-cluster",
			Name:      fmt.Sprintf(upgradeJobName, in.Name),
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: "embedded-cluster-operator",
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: fmt.Sprintf(upgradeJobConfigMap, in.Name),
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "embedded-cluster-updater",
							Image: operatorImage,
							Command: []string{
								"/manager",
								"upgrade-job",
								"--installation",
								"/config/installation.yaml",
								"--local-artifact-mirror-image",
								localArtifactMirrorImage,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/config",
								},
							},
						},
					},
				},
			},
		},
	}
	if err = cli.Create(ctx, job); err != nil {
		return fmt.Errorf("failed to create upgrade job: %w", err)
	}

	return nil
}

func operatorImageName(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) (string, error) {
	// determine the image to use for the upgrade job
	meta, err := release.MetadataFor(ctx, in, cli)
	if err != nil {
		return "", fmt.Errorf("failed to get release metadata: %w", err)
	}
	operatorImage := ""
	for _, image := range meta.Images {
		if strings.Contains(image, "embedded-cluster-operator-image") {
			operatorImage = image
			break
		}
	}
	return operatorImage, nil
}

// Upgrade upgrades the embedded cluster to the version specified in the installation.
// First the k0s cluster is upgraded, then addon charts are upgraded, and finally the installation is created.
func Upgrade(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation, localArtifactMirrorImage string) error {
	previousInstall, err := getPreviousInstallation(ctx, cli)
	if err != nil {
		return fmt.Errorf("get current installation: %w", err)
	}

	err = k0sUpgrade(ctx, cli, in, previousInstall)
	if err != nil {
		return fmt.Errorf("k0s upgrade: %w", err)
	}

	err = chartUpgrade(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("chart upgrade: %w", err)
	}

	// wait for the operator chart to be ready
	err = waitForOperatorChart(ctx, cli, in.Spec.Config.Version)
	if err != nil {
		return fmt.Errorf("wait for operator chart: %w", err)
	}

	err = unLockInstallation(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("unlock installation: %w", err)
	}

	return nil
}

func k0sUpgrade(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation, currentInstall *clusterv1beta1.Installation) error {
	meta, err := release.MetadataFor(ctx, in, cli)
	if err != nil {
		return fmt.Errorf("failed to get release metadata: %w", err)
	}

	// TODO: THIS IS NOT THE RIGHT WAY TO GET THE K0S VERSION
	// it fails if we're currently upgrading, after all
	currentMeta, err := release.MetadataFor(ctx, currentInstall, cli)
	if err != nil {
		return fmt.Errorf("failed to get current release metadata: %w", err)
	}

	// check if the k0s version is the same as the current version
	// if it is, we can skip the upgrade
	desiredVersion := meta.Versions["Kubernetes"]
	currentVersion := currentMeta.Versions["Kubernetes"]
	if desiredVersion == currentVersion {
		return nil
	}

	//// if the current version is < 1.30 and the desired version is >= 1.30, we need to remove 'timeout: 0' from the /etc/k0s/k0s.yaml
	//// file on each controller node. This is because the format changed in 1.30 and expects this value to be a string instead of an integer.
	//if semver.Compare(currentVersion, "v1.30.0") < 0 && semver.Compare(desiredVersion, "v1.30.0") >= 0 {
	//	err = removeTimeoutFromK0sConfig(ctx, cli, currentInstall)
	//	if err != nil {
	//		return fmt.Errorf("failed to remove timeout from k0s config: %w", err)
	//	}
	//}

	// create an autopilot upgrade plan if one does not yet exist
	var plan apv1b2.Plan
	okey := client.ObjectKey{Name: "autopilot"}
	if err := cli.Get(ctx, okey, &plan); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get upgrade plan: %w", err)
	} else if errors.IsNotFound(err) {
		// if the kubernetes version has changed we create an upgrade command
		fmt.Printf("Starting k0s autopilot upgrade plan to version %s\n", desiredVersion)

		// there is no autopilot plan in the cluster so we are free to
		// start our own plan. here we link the plan to the installation
		// by its name.
		if err := StartAutopilotUpgrade(ctx, cli, in, meta); err != nil {
			return fmt.Errorf("failed to start upgrade: %w", err)
		}
	}

	// restart this function/pod until the plan is complete
	if !autopilot.HasThePlanEnded(plan) {
		return fmt.Errorf("an autopilot upgrade is in progress (%s)", plan.Spec.ID)
	}

	if autopilot.HasPlanFailed(plan) {
		reason := autopilot.ReasonForState(plan)
		return fmt.Errorf("autopilot plan failed: %s", reason)
	}

	// the plan has been completed, so we can move on - kubernetes is now upgraded
	fmt.Printf("Upgrade to %s completed successfully\n", desiredVersion)
	if err := cli.Delete(ctx, &plan); err != nil {
		return fmt.Errorf("failed to delete successful upgrade plan: %w", err)
	}
	return nil
}

// run a pod on each controller node to remove the timeout from the k0s config file
func removeTimeoutFromK0sConfig(ctx context.Context, cli client.Client, install *clusterv1beta1.Installation) error {
	// get the list of controller nodes
	var nodes corev1.NodeList
	if err := cli.List(ctx, &nodes); err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	controllerNodes := []string{}
	for _, node := range nodes.Items {
		// run a pod on each controller node to remove the timeout from the k0s config file
		if node.Labels["node-role.kubernetes.io/control-plane"] == "true" {
			controllerNodes = append(controllerNodes, node.Name)
		}
	}

	operatorImage, err := operatorImageName(ctx, cli, install)
	if err != nil {
		return fmt.Errorf("failed to get operator image name: %w", err)
	}

	fmt.Printf("Removing timeout from k0s config on controller nodes: %v\n", controllerNodes)

	// run a job on each controller node to remove the timeout from the k0s config file
	for _, node := range controllerNodes {
		// create a job to remove the timeout from the k0s config file
		j := batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("remove-timeout-%s", node),
				Namespace: "embedded-cluster",
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						NodeName:      node,
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser: ptr.To[int64](0),
						},
						Containers: []corev1.Container{
							{
								Name:  "remove-timeout",
								Image: operatorImage,
								Command: []string{
									"/bin/sh",
									"-c",
									"sed 's/timeout: 0//g' /sed > /tmp/k0s.yaml && cat /tmp/k0s.yaml > /sed",
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "config",
										MountPath: "/sed",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "config",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/etc/k0s/k0s.yaml",
										Type: ptr.To[corev1.HostPathType]("File"),
									},
								},
							},
						},
					},
				},
			},
		}

		err = cli.Create(ctx, &j)
		if err != nil {
			return fmt.Errorf("failed to create pod: %w", err)
		}
	}

	fmt.Printf("Waiting for jobs to complete on controller nodes: %v\n", controllerNodes)

	i := 0
	for {
		time.Sleep(5 * time.Second)
		jobNames := []string{}
		for _, node := range controllerNodes {
			jobNames = append(jobNames, fmt.Sprintf("remove-timeout-%s", node))
		}

		if jobsAllCompleted(ctx, cli, jobNames, "embedded-cluster") {
			break
		}
		if i == 4 {
			return fmt.Errorf("timed out waiting for jobs to complete")
		}
		i++
	}

	// remove the jobs from the cluster
	for _, node := range controllerNodes {
		j := batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("remove-timeout-%s", node),
				Namespace: "embedded-cluster",
			},
		}
		err = cli.Delete(ctx, &j)
		if err != nil {
			return fmt.Errorf("failed to delete job: %w", err)
		}
	}

	return nil
}

func jobsAllCompleted(ctx context.Context, cli client.Client, jobs []string, ns string) bool {
	// get all the jobs in the list, and check if they are completed
	var jobList batchv1.JobList
	if err := cli.List(ctx, &jobList, client.InNamespace(ns)); err != nil {
		return false
	}

	for _, j := range jobs {
		found := false
		for _, job := range jobList.Items {
			if job.Name == j {
				if job.Status.Succeeded == 0 {
					fmt.Printf("Job %s is not completed\n", job.Name)
					return false
				}
				found = true
			}
		}
		if !found {
			fmt.Printf("Job %s not found\n", j)
			return false
		}
	}
	return true
}

// copied from ReconcileHelmCharts in https://github.com/replicatedhq/embedded-cluster/blob/c6a57a4/operator/controllers/installation_controller.go#L568
func chartUpgrade(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) error {
	meta, err := release.MetadataFor(ctx, in, cli)
	if err != nil {
		return fmt.Errorf("failed to get release metadata: %w", err)
	}

	// fetch the current clusterConfig
	var clusterConfig k0sv1beta1.ClusterConfig
	if err := cli.Get(ctx, client.ObjectKey{Name: "k0s", Namespace: "kube-system"}, &clusterConfig); err != nil {
		return fmt.Errorf("failed to get cluster config: %w", err)
	}

	combinedConfigs, err := charts.K0sHelmExtensionsFromInstallation(ctx, in, meta, &clusterConfig)
	if err != nil {
		return fmt.Errorf("failed to get helm charts from installation: %w", err)
	}

	cfgs := &k0sv1beta1.HelmExtensions{}
	cfgs, err = v1beta1.ConvertTo(*combinedConfigs, cfgs)
	if err != nil {
		return fmt.Errorf("failed to convert chart types: %w", err)
	}

	existingHelm := &k0sv1beta1.HelmExtensions{}
	if clusterConfig.Spec != nil && clusterConfig.Spec.Extensions != nil && clusterConfig.Spec.Extensions.Helm != nil {
		existingHelm = clusterConfig.Spec.Extensions.Helm
	}

	chartDrift, changedCharts, err := charts.DetectChartDrift(cfgs, existingHelm)
	if err != nil {
		return fmt.Errorf("failed to check chart drift: %w", err)
	}

	// detect drift between the cluster config and the installer metadata
	var installedCharts k0shelm.ChartList
	if err := cli.List(ctx, &installedCharts); err != nil {
		return fmt.Errorf("failed to list installed charts: %w", err)
	}
	pendingCharts, chartErrors, err := charts.DetectChartCompletion(existingHelm, installedCharts)
	if err != nil {
		return fmt.Errorf("failed to check chart completion: %w", err)
	}

	// if there is a difference between what we want and what we have
	// we should update the cluster instead of letting chart errors stop deployment permanently
	// otherwise if there are errors we need to abort
	if len(chartErrors) > 0 && !chartDrift {
		chartErrorString := strings.Join(chartErrors, ",")
		chartErrorString = "failed to update helm charts: " + chartErrorString
		fmt.Printf("Chart errors: %s\n", chartErrorString)
		return fmt.Errorf("helm charts have errors and there is no update to be applied")
	}

	// If all addons match their target version + values, things are successful
	// This should not happen on upgrades
	if len(pendingCharts) == 0 && !chartDrift {
		return nil
	}

	if len(pendingCharts) > 0 {
		// If there are pending charts, return an error because we need to wait for some prior installation to complete
		return fmt.Errorf("pending charts: %v", pendingCharts)
	}

	if !chartDrift {
		// if there is no drift, we should not reapply the cluster config
		// This should not happen on upgrades
		return nil
	}

	// Replace the current chart configs with the new chart configs
	clusterConfig.Spec.Extensions.Helm = cfgs
	fmt.Printf("Updating cluster config with new helm charts %v\n", changedCharts)
	//Update the clusterConfig
	if err := cli.Update(ctx, &clusterConfig); err != nil {
		return fmt.Errorf("failed to update cluster config: %w", err)
	}
	return nil

}

func createInstallation(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) error {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Creating installation...")

	// the installation always has a unique name (current timestamp), so we can just create it

	err := cli.Create(ctx, in)
	if err != nil {
		return fmt.Errorf("create installation: %w", err)
	}

	// set the state to 'waiting' so that the operator will not reconcile based on it
	// we will set the state to installed after the installation is complete
	in.Status.State = v1beta1.InstallationStateWaiting
	err = cli.Status().Update(ctx, in)
	if err != nil {
		return fmt.Errorf("update installation status: %w", err)
	}

	log.Info("Installation created")

	return nil
}

func unLockInstallation(ctx context.Context, cli client.Client, in *v1beta1.Installation) error {
	// if the installation is locked, we need to unlock it
	if in.Status.State == v1beta1.InstallationStateWaiting {
		in.Status.State = v1beta1.InstallationStateKubernetesInstalled
		err := cli.Status().Update(ctx, in)
		if err != nil {
			return fmt.Errorf("update installation status: %w", err)
		}
	}
	return nil
}

func getPreviousInstallation(ctx context.Context, cli client.Client) (*v1beta1.Installation, error) {
	var installList v1beta1.InstallationList
	if err := cli.List(ctx, &installList); err != nil {
		return nil, fmt.Errorf("list installations: %w", err)
	}

	// sort the installations by installation name
	sort.SliceStable(installList.Items, func(i, j int) bool {
		return installList.Items[i].Name < installList.Items[j].Name
	})

	if len(installList.Items) < 2 {
		return nil, fmt.Errorf("only %d installations found, expected 2+", len(installList.Items))
	}

	return &installList.Items[1], nil
}

func waitForOperatorChart(ctx context.Context, cli client.Client, version string) error {
	err := wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		ready, err := k8sutil.GetChartHealthVersion(ctx, cli, operatorChartName, version)
		if err != nil {
			return false, fmt.Errorf("get chart health: %w", err)
		}
		return ready, nil
	})
	return err
}

const (
	installationNameAnnotation = "embedded-cluster.replicated.com/installation-name"
)

func airgapDistributeArtifacts(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation, localArtifactMirrorImage string) error {
	// in airgap installations let's make sure all assets have been copied to nodes.
	// this may take some time so we only move forward when 'ready'.
	err := ensureAirgapArtifactsOnNodes(ctx, cli, in, localArtifactMirrorImage)
	if err != nil {
		return fmt.Errorf("ensure airgap artifacts: %w", err)
	}

	// once all assets are in place we can create the autopilot plan to push the images to
	// containerd.
	err = ensureAirgapArtifactsInCluster(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("autopilot copy airgap artifacts: %w", err)
	}

	return nil
}

func ensureAirgapArtifactsOnNodes(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation, localArtifactMirrorImage string) error {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Placing artifacts on nodes...")

	op, err := artifacts.EnsureRegistrySecretInECNamespace(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("ensure registry secret in ec namespace: %w", err)
	} else if op != controllerutil.OperationResultNone {
		log.Info("Registry credentials secret changed", "operation", op)
	}

	err = artifacts.EnsureArtifactsJobForNodes(ctx, cli, in, localArtifactMirrorImage)
	if err != nil {
		return fmt.Errorf("ensure artifacts job for nodes: %w", err)
	}

	log.Info("Waiting for artifacts to be placed on nodes...")

	err = wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		jobs, err := artifacts.ListArtifactsJobForNodes(ctx, cli, in)
		if err != nil {
			return false, fmt.Errorf("list artifacts jobs for nodes: %w", err)
		}

		ready := true
		for nodeName, job := range jobs {
			if job == nil {
				return false, fmt.Errorf("job for node %s not found", nodeName)
			}
			if job.Status.Succeeded > 0 {
				continue
			}
			ready = false
			for _, cond := range job.Status.Conditions {
				if cond.Type == batchv1.JobFailed {
					if cond.Status == corev1.ConditionTrue {
						// fail immediately if any job fails
						return false, fmt.Errorf("job for node %s failed: %s - %s", nodeName, cond.Reason, cond.Message)
					}
					break
				}
			}
			// job is still running
		}

		return ready, nil
	})
	if err != nil {
		return fmt.Errorf("wait for artifacts job for nodes: %w", err)
	}

	log.Info("Artifacts placed on nodes")
	return nil
}

func ensureAirgapArtifactsInCluster(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) error {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Uploading container images...")

	err := autopilotEnsureAirgapArtifactsPlan(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("ensure autopilot plan: %w", err)
	}

	nsn := types.NamespacedName{Name: "autopilot"}
	plan := autopilotv1beta2.Plan{}

	log.Info("Waiting for container images to be uploaded...")

	err = wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		err := cli.Get(ctx, nsn, &plan)
		if err != nil {
			return false, fmt.Errorf("get autopilot plan: %w", err)
		}
		if plan.Annotations[installationNameAnnotation] != in.Name {
			return false, fmt.Errorf("autopilot plan for different installation")
		}

		switch {
		case autopilot.HasPlanSucceeded(plan):
			return true, nil
		case autopilot.HasPlanFailed(plan):
			reason := autopilot.ReasonForState(plan)
			return false, fmt.Errorf("autopilot plan failed: %s", reason)
		}
		// plan is still running
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("wait for autopilot plan: %w", err)
	}

	log.Info("Container images uploaded")
	return nil
}

func autopilotEnsureAirgapArtifactsPlan(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) error {
	plan, err := getAutopilotAirgapArtifactsPlan(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("get autopilot airgap artifacts plan: %w", err)
	}

	err = k8sutil.EnsureObject(ctx, cli, plan, func(opts *k8sutil.EnsureObjectOptions) {
		opts.ShouldDelete = func(obj client.Object) bool {
			return obj.GetAnnotations()[installationNameAnnotation] != in.Name
		}
	})
	if err != nil {
		return fmt.Errorf("ensure autopilot plan: %w", err)
	}

	return nil
}

func getAutopilotAirgapArtifactsPlan(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) (*autopilotv1beta2.Plan, error) {
	var commands []autopilotv1beta2.PlanCommand

	// if we are running in an airgap environment all assets are already present in the
	// node and are served by the local-artifact-mirror binary listening on localhost
	// port 50000. we just need to get autopilot to fetch the k0s binary from there.
	command, err := artifacts.CreateAutopilotAirgapPlanCommand(ctx, cli, in)
	if err != nil {
		return nil, fmt.Errorf("create autopilot airgap plan command: %w", err)
	}
	commands = append(commands, *command)

	plan := &autopilotv1beta2.Plan{
		TypeMeta: metav1.TypeMeta{
			APIVersion: autopilotv1beta2.SchemeGroupVersion.String(),
			Kind:       "Plan",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot", // this is a fixed name and should not be changed
			Annotations: map[string]string{
				installationNameAnnotation: in.Name,
			},
		},
		Spec: autopilotv1beta2.PlanSpec{
			Timestamp: "now",
			ID:        uuid.New().String(),
			Commands:  commands,
		},
	}

	return plan, nil
}
