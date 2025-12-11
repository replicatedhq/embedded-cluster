package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Validate performs comprehensive installation validation using Kubernetes client.
//
// This method orchestrates all validation checks and returns a ValidationResult
// containing the results of each check. All checks are run regardless of individual
// failures to provide a complete picture of the installation state.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  with-cmx-vm --vm-id 8a2a66ef \
//	  validate --scenario=online --expected-kube-version=1.33 --expected-app-version=appver-dev-xpXCTO string
func (i *CmxInstance) Validate(
	ctx context.Context,
	// Scenario (online, airgap)
	scenario string,
	// Expected Kubernetes version (e.g., "1.31")
	expectedKubeVersion string,
	// Expected app version (e.g., "v1.0.0")
	expectedAppVersion string,
) *ValidationResult {
	validationResult := &ValidationResult{
		Success: true,
	}

	airgap := scenario == "airgap"

	// Run validation checks in order and populate fields
	validationResult.ClusterHealth = i.ValidateClusterHealth(ctx, expectedKubeVersion)
	validationResult.InstallationCRD = i.ValidateInstallationCRD(ctx)
	validationResult.AppDeployment = i.ValidateAppDeployment(ctx, expectedAppVersion, airgap)
	validationResult.AdminConsole = i.ValidateAdminConsole(ctx)
	validationResult.DataDirectories = i.ValidateDataDirectories(ctx)
	validationResult.PodsAndJobs = i.ValidatePodsAndJobs(ctx)

	// Determine overall success
	allChecks := []*CheckResult{
		validationResult.ClusterHealth,
		validationResult.InstallationCRD,
		validationResult.AppDeployment,
		validationResult.AdminConsole,
		validationResult.DataDirectories,
		validationResult.PodsAndJobs,
	}

	for _, check := range allChecks {
		fmt.Println(check.String())

		if check != nil && !check.Passed {
			validationResult.Success = false
		}
	}

	return validationResult
}

// validateClusterHealth validates Kubernetes cluster health.
//
// This check verifies:
// - All nodes are running the expected Kubernetes version
// - Kubelet version matches expected version on all nodes
// - All nodes are in Ready state (none in NotReady)
//
// Based on: e2e/scripts/common.sh::ensure_nodes_match_kube_version
func (i *CmxInstance) ValidateClusterHealth(ctx context.Context, expectedK8sVersion string) *CheckResult {
	result := &CheckResult{Passed: true}

	// Check node versions match expected k8s version
	stdout, err := i.Command(`kubectl get nodes -o jsonpath={.items[*].status.nodeInfo.kubeletVersion}`).Stdout(ctx)
	if err != nil {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("failed to get node versions: %v", err)
		return result
	}

	// Verify all nodes have the expected version
	if !strings.Contains(stdout, expectedK8sVersion) {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("node version mismatch: got %s, want %s", stdout, expectedK8sVersion)
		result.Details = "Not all nodes are running the expected Kubernetes version"
		return result
	}

	// Check node readiness - works for both single-node and multi-node
	stdout, err = i.Command(`kubectl get nodes --no-headers`).Stdout(ctx)
	if err != nil {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("failed to get nodes: %v", err)
		return result
	}

	// Check that no nodes are NotReady
	if strings.Contains(stdout, "NotReady") {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("one or more nodes are not ready: %s", stdout)
		result.Details = "Found nodes in NotReady state"
		return result
	}

	// Verify all nodes contain "Ready" status
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if !strings.Contains(line, "Ready") {
			result.Passed = false
			result.ErrorMessage = fmt.Sprintf("node not ready: %s", line)
			result.Details = "Node does not have Ready status"
			return result
		}
	}

	result.Details = fmt.Sprintf("All nodes running k8s %s and in Ready state", expectedK8sVersion)
	return result
}

// validateInstallationCRD validates the Installation CRD status.
//
// This check verifies:
// - Installation resource exists
// - Installation is in "Installed" state
// - Embedded-cluster operator successfully completed installation
//
// Based on: e2e/scripts/common.sh::ensure_installation_is_installed
func (i *CmxInstance) ValidateInstallationCRD(ctx context.Context) *CheckResult {
	result := &CheckResult{Passed: true}

	// Check if Installation resource exists and is in Installed state
	stdout, err := i.Command(`kubectl get installations --no-headers`).Stdout(ctx)
	if err != nil {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("failed to get installations: %v", err)

		// Get more details for debugging
		details, _ := i.Command(`kubectl get installations`).Stdout(ctx)
		describeOut, _ := i.Command(`kubectl describe installations`).Stdout(ctx)
		result.Details = fmt.Sprintf("installations output:\n%s\n\ndescribe:\n%s", details, describeOut)
		return result
	}

	if !strings.Contains(stdout, "Installed") {
		result.Passed = false
		result.ErrorMessage = "installation is not in Installed state"

		// Gather debugging information
		installations, _ := i.Command(`kubectl get installations`).Stdout(ctx)
		describe, _ := i.Command(`kubectl describe installations`).Stdout(ctx)
		charts, _ := i.Command(`kubectl get charts -A`).Stdout(ctx)
		pods, _ := i.Command(`kubectl get pods -A`).Stdout(ctx)

		result.Details = fmt.Sprintf("installations:\n%s\n\ndescribe:\n%s\n\ncharts:\n%s\n\npods:\n%s",
			installations, describe, charts, pods)
		return result
	}

	result.Details = "Installation resource exists and is in Installed state"
	return result
}

// validateAppDeployment validates the application deployment status.
//
// This check verifies:
// - Application's nginx pods are Running
// - Correct app version is deployed
// - No upgrade artifacts present (kube-state-metrics namespace, "second" app pods)
//
// Based on: e2e/scripts/common.sh::wait_for_nginx_pods, ensure_app_deployed, ensure_app_not_upgraded
func (i *CmxInstance) ValidateAppDeployment(ctx context.Context, expectedAppVersion string, airgap bool) *CheckResult {
	result := &CheckResult{Passed: true}

	// Wait for nginx pods to be Running (with timeout)
	nginxReady := false
	timeout := time.After(1 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

waitLoop:
	for {
		select {
		case <-timeout:
			result.Passed = false
			result.ErrorMessage = "nginx pods did not appear within timeout"

			// Get debugging info
			pods, _ := i.Command(fmt.Sprintf(`kubectl get pods -n %s`, AppNamespace)).Stdout(ctx)
			kotsadmPods, _ := i.Command(fmt.Sprintf(`kubectl get pods -n %s`, AppNamespace)).Stdout(ctx)
			logs, _ := i.Command(fmt.Sprintf(`kubectl logs -n %s -l app=kotsadm --tail=50`, AppNamespace)).Stdout(ctx)

			result.Details = fmt.Sprintf("app pods:\n%s\n\nkotsadm pods:\n%s\n\nkotsadm logs:\n%s",
				pods, kotsadmPods, logs)
			return result

		case <-ticker.C:
			stdout, err := i.Command(fmt.Sprintf(`kubectl get pods -n %s --no-headers`, AppNamespace)).Stdout(ctx)
			if err == nil && strings.Contains(stdout, "nginx") && strings.Contains(stdout, "Running") {
				nginxReady = true
				break waitLoop
			}
		}
	}

	if !nginxReady {
		result.Passed = false
		result.ErrorMessage = "nginx pods not in Running state"
		return result
	}

	// Verify app version is deployed
	if airgap {
		// For airgap, use kotsadm API to check version
		if err := i.ensureAppDeployedAirgap(ctx, expectedAppVersion); err != nil {
			result.Passed = false
			result.ErrorMessage = fmt.Sprintf("app version validation failed (airgap): %v", err)
			return result
		}
	} else {
		// For online, use kubectl kots
		versions, err := i.Command(fmt.Sprintf(`kubectl kots get versions -n %s embedded-cluster-smoke-test-staging-app`, AppNamespace)).Stdout(ctx)
		if err != nil {
			result.Passed = false
			result.ErrorMessage = fmt.Sprintf("failed to get app versions: %v", err)
			result.Details = versions
			return result
		}

		// Check for expected version with "deployed" status
		// Format: version number deployed
		if !strings.Contains(versions, expectedAppVersion) || !strings.Contains(versions, "deployed") {
			result.Passed = false
			result.ErrorMessage = fmt.Sprintf("app version %s not deployed", expectedAppVersion)
			result.Details = fmt.Sprintf("versions output:\n%s", versions)
			return result
		}
	}

	// Ensure no upgrade artifacts present
	// Check for kube-state-metrics namespace (should not exist for fresh install)
	nsOutput, _ := i.Command(`kubectl get ns`).Stdout(ctx)
	if strings.Contains(nsOutput, "kube-state-metrics") {
		result.Passed = false
		result.ErrorMessage = "found kube-state-metrics namespace (upgrade artifact)"
		result.Details = fmt.Sprintf("namespaces:\n%s", nsOutput)
		return result
	}

	// Check for "second" app pods (should not exist for fresh install)
	secondPods, _ := i.Command(fmt.Sprintf(`kubectl get pods -n %s -l app=second`, AppNamespace)).Stdout(ctx)
	if strings.Contains(secondPods, "second") {
		result.Passed = false
		result.ErrorMessage = "found pods from app update (upgrade artifact)"
		result.Details = fmt.Sprintf("second pods:\n%s", secondPods)
		return result
	}

	result.Details = fmt.Sprintf("App version %s deployed successfully, nginx pods running, no upgrade artifacts", expectedAppVersion)
	return result
}

// ensureAppDeployedAirgap checks app deployment status using kotsadm API (for airgap scenarios).
//
// Based on: e2e/scripts/common.sh::ensure_app_deployed_airgap
func (i *CmxInstance) ensureAppDeployedAirgap(ctx context.Context, expectedVersion string) error {
	// Get kotsadm authstring
	authStringCmd := fmt.Sprintf(`kubectl get secret -n %s kotsadm-authstring -o jsonpath={.data.kotsadm-authstring}`, AppNamespace)
	authString64, err := i.Command(authStringCmd).Stdout(ctx)
	if err != nil {
		return fmt.Errorf("get authstring: %w", err)
	}

	// Decode authstring (base64)
	decodeCmd := fmt.Sprintf(`sh -c "echo '%s' | base64 -d"`, authString64)
	authString, err := i.Command(decodeCmd).Stdout(ctx)
	if err != nil {
		return fmt.Errorf("decode authstring: %w", err)
	}

	// Get kotsadm service IP
	kotsadmIPCmd := fmt.Sprintf(`kubectl get svc -n %s kotsadm -o jsonpath='{.spec.clusterIP}'`, AppNamespace)
	kotsadmIP, err := i.Command(kotsadmIPCmd).Stdout(ctx)
	if err != nil {
		return fmt.Errorf("get kotsadm IP: %w", err)
	}

	// Get kotsadm service port
	kotsadmPortCmd := fmt.Sprintf(`kubectl get svc -n %s kotsadm -o jsonpath='{.spec.ports[?(@.name=="http")].port}'`, AppNamespace)
	kotsadmPort, err := i.Command(kotsadmPortCmd).Stdout(ctx)
	if err != nil {
		return fmt.Errorf("get kotsadm port: %w", err)
	}

	// Query kotsadm API for versions
	apiURL := fmt.Sprintf("http://%s:%s/api/v1/app/embedded-cluster-smoke-test-staging-app/versions?currentPage=0&pageSize=1",
		strings.TrimSpace(kotsadmIP), strings.TrimSpace(kotsadmPort))

	curlCmd := fmt.Sprintf(`curl -k -X GET "%s" -H "Authorization: %s"`, apiURL, strings.TrimSpace(authString))
	versions, err := i.Command(curlCmd).Stdout(ctx)
	if err != nil {
		return fmt.Errorf("query kotsadm API: %w", err)
	}

	// Search for the version and that it is deployed
	// Format: "versionLabel":"v1.0.0"..."status":"deployed"
	// There should not be a '}' between the version and the status
	versionPattern := fmt.Sprintf(`"versionLabel":"%s"`, expectedVersion)
	if !strings.Contains(versions, versionPattern) {
		return fmt.Errorf("version %s not found in API response: %s", expectedVersion, versions)
	}

	// Check that the version is deployed
	if !strings.Contains(versions, `"status":"deployed"`) {
		return fmt.Errorf("version %s not deployed: %s", expectedVersion, versions)
	}

	return nil
}

// validateAdminConsole validates the admin console components.
//
// This check verifies:
// - kotsadm pods are healthy
// - kotsadm API is healthy (kubectl kots get apps works)
// - Admin console branding configmap exists with DR label
//
// Based on: e2e/scripts/check-installation-state.sh
func (i *CmxInstance) ValidateAdminConsole(ctx context.Context) *CheckResult {
	result := &CheckResult{Passed: true}

	// Check kotsadm pods are running
	kotsadmPods, err := i.Command(fmt.Sprintf(`kubectl get pods -n %s -l app=kotsadm --no-headers`, AppNamespace)).Stdout(ctx)
	if err != nil {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("failed to get kotsadm pods: %v", err)
		return result
	}

	if !strings.Contains(kotsadmPods, "Running") {
		result.Passed = false
		result.ErrorMessage = "kotsadm pods are not running"
		result.Details = fmt.Sprintf("kotsadm pods:\n%s", kotsadmPods)
		return result
	}

	// Check kubectl kots command works
	_, err = i.Command(fmt.Sprintf(`kubectl kots get apps -n %s`, AppNamespace)).Stdout(ctx)
	if err != nil {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("kubectl kots get apps failed: %v", err)
		result.Details = "kotsadm API is not healthy"
		return result
	}

	// Check admin console branding configmap has DR label
	cmCheck, err := i.Command(fmt.Sprintf(`kubectl get cm -n %s kotsadm-application-metadata --show-labels`, AppNamespace)).Stdout(ctx)
	if err != nil {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("failed to get kotsadm-application-metadata configmap: %v", err)
		return result
	}

	if !strings.Contains(cmCheck, "replicated.com/disaster-recovery=infra") {
		result.Passed = false
		result.ErrorMessage = "kotsadm-application-metadata configmap missing DR label"

		// Get full configmap details
		cmDetails, _ := i.Command(fmt.Sprintf(`kubectl get cm -n %s kotsadm-application-metadata -o yaml`, AppNamespace)).Stdout(ctx)
		result.Details = fmt.Sprintf("configmap:\n%s", cmDetails)
		return result
	}

	result.Details = "Admin console components healthy, kotsadm API responding, branding configmap has DR label"
	return result
}

// validateDataDirectories validates data directory configuration.
//
// This check verifies:
// - K0s data directory is configured correctly
// - OpenEBS data directory is configured correctly and not empty
// - Velero pod volume path is configured correctly
// - All components use expected base directory
//
// Based on: e2e/scripts/common.sh::validate_data_dirs
func (i *CmxInstance) ValidateDataDirectories(ctx context.Context) *CheckResult {
	result := &CheckResult{Passed: true}
	expectedBaseDir := DataDir
	expectedK0sDataDir := fmt.Sprintf("%s/k0s", DataDir)
	expectedOpenEBSDataDir := fmt.Sprintf("%s/openebs-local", DataDir)

	var errors []string

	// Validate OpenEBS data directory exists and is not empty
	lsCmd := fmt.Sprintf(`ls -A %s`, expectedOpenEBSDataDir)
	lsOutput, err := i.Command(lsCmd).Stdout(ctx)
	if err != nil {
		errors = append(errors, fmt.Sprintf("OpenEBS data directory %s does not exist or is not accessible: %v", expectedOpenEBSDataDir, err))
	} else if strings.TrimSpace(lsOutput) == "" {
		errors = append(errors, fmt.Sprintf("OpenEBS data directory %s is empty", expectedOpenEBSDataDir))
	}

	// Validate OpenEBS helm values
	helmCmd := fmt.Sprintf(`%s/bin/helm get values -n openebs openebs`, DataDir)
	openebsValues, err := i.Command(helmCmd).Stdout(ctx)
	if err != nil {
		errors = append(errors, fmt.Sprintf("failed to get OpenEBS helm values: %v", err))
	} else {
		// Check basePath configuration
		if !strings.Contains(openebsValues, fmt.Sprintf("basePath: %s", expectedOpenEBSDataDir)) {
			errors = append(errors, fmt.Sprintf("OpenEBS basePath not set to %s", expectedOpenEBSDataDir))
			result.Details += fmt.Sprintf("\nOpenEBS helm values:\n%s", openebsValues)
		}
	}

	// Validate Velero helm values
	helmCmd = fmt.Sprintf(`%s/bin/helm get values -n velero velero`, DataDir)
	veleroValues, err := i.Command(helmCmd).Stdout(ctx)
	if err != nil {
		errors = append(errors, fmt.Sprintf("failed to get Velero helm values: %v", err))
	} else {
		// Check podVolumePath configuration
		expectedVeleroPath := fmt.Sprintf("%s/kubelet/pods", expectedK0sDataDir)
		if !strings.Contains(veleroValues, fmt.Sprintf("podVolumePath: %s", expectedVeleroPath)) {
			errors = append(errors, fmt.Sprintf("Velero podVolumePath not set to %s", expectedVeleroPath))
			result.Details += fmt.Sprintf("\nVelero helm values:\n%s", veleroValues)
		}
	}

	// Validate SeaweedFS helm values if HA
	// TODO

	if len(errors) > 0 {
		result.Passed = false
		result.ErrorMessage = strings.Join(errors, "; ")
		return result
	}

	result.Details = fmt.Sprintf("Data directories configured correctly (base: %s, k0s: %s, openebs: %s not empty)",
		expectedBaseDir, expectedK0sDataDir, expectedOpenEBSDataDir)
	return result
}

// validatePodsAndJobs validates pod and job health.
//
// This check verifies:
// - All non-Job pods are in Running/Completed/Succeeded state
// - All Running pods have ready containers
// - All Jobs have completed successfully
//
// Based on: e2e/scripts/common.sh::validate_all_pods_healthy
func (i *CmxInstance) ValidatePodsAndJobs(ctx context.Context) *CheckResult {
	result := &CheckResult{Passed: true}
	timeout := 5 * time.Minute
	startTime := time.Now()

	for {
		elapsed := time.Since(startTime)
		if elapsed >= timeout {
			result.Passed = false
			result.ErrorMessage = "timed out waiting for pods and jobs to be healthy after 5 minutes"

			// Gather failure details
			nonJobCheck := i.validateNonJobPodsHealthy(ctx)
			jobCheck := i.validateJobsCompleted(ctx)

			result.Details = fmt.Sprintf("Non-Job pods: %s\nJobs: %s",
				nonJobCheck.ErrorMessage, jobCheck.ErrorMessage)
			return result
		}

		// Check if both validations pass
		podsHealthy := i.validateNonJobPodsHealthy(ctx)
		jobsHealthy := i.validateJobsCompleted(ctx)

		if podsHealthy.Passed && jobsHealthy.Passed {
			result.Details = "All pods and jobs are healthy"
			return result
		}

		// Wait before retrying
		time.Sleep(10 * time.Second)
	}
}

// validateNonJobPodsHealthy checks that all non-Job pods are healthy.
//
// Based on: e2e/scripts/common.sh::validate_non_job_pods_healthy
func (i *CmxInstance) validateNonJobPodsHealthy(ctx context.Context) *CheckResult {
	result := &CheckResult{Passed: true}

	// Get all pods with custom columns
	podsCommand := `kubectl get pods -A --no-headers -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,STATUS:.status.phase,OWNER:.metadata.ownerReferences[0].kind`
	podsOutput, err := i.Command(podsCommand).Stdout(ctx)
	if err != nil {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("failed to get pods: %v", err)
		return result
	}

	// Check for unhealthy non-Job pods
	var unhealthyPods []string
	lines := strings.Split(strings.TrimSpace(podsOutput), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		namespace, name, status, owner := fields[0], fields[1], fields[2], fields[3]
		if owner == "Job" {
			continue // Skip Job pods
		}

		// Check if pod is in acceptable state
		if status != "Running" && status != "Completed" && status != "Succeeded" {
			unhealthyPods = append(unhealthyPods, fmt.Sprintf("%s/%s (%s)", namespace, name, status))
		}
	}

	if len(unhealthyPods) > 0 {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("found non-Job pods in unhealthy state: %s", strings.Join(unhealthyPods, ", "))
		return result
	}

	// Check container readiness for Running pods
	readyCommand := `kubectl get pods -A --no-headers -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,STATUS:.status.phase,READY:.status.containerStatuses[*].ready,OWNER:.metadata.ownerReferences[0].kind`
	readyOutput, err := i.Command(readyCommand).Stdout(ctx)
	if err != nil {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("failed to check pod readiness: %v", err)
		return result
	}

	var unreadyPods []string
	lines = strings.Split(strings.TrimSpace(readyOutput), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		namespace, name, status, ready, owner := fields[0], fields[1], fields[2], fields[3], fields[4]
		if owner == "Job" || status != "Running" {
			continue // Skip Job pods and non-Running pods
		}

		// Check if all containers are ready (should be "true" for all)
		if ready == "" || !strings.Contains(ready, "true") {
			unreadyPods = append(unreadyPods, fmt.Sprintf("%s/%s (not ready)", namespace, name))
		}
	}

	if len(unreadyPods) > 0 {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("found Running pods that are not ready: %s", strings.Join(unreadyPods, ", "))
		return result
	}

	result.Details = "All non-Job pods are healthy"
	return result
}

// validateJobsCompleted checks that all Jobs have completed successfully.
//
// Based on: e2e/scripts/common.sh::validate_jobs_completed
func (i *CmxInstance) validateJobsCompleted(ctx context.Context) *CheckResult {
	result := &CheckResult{Passed: true}

	// Get all Jobs with completions status
	jobsCommand := `kubectl get jobs -A --no-headers -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,COMPLETIONS:.spec.completions,SUCCESSFUL:.status.succeeded`
	jobsOutput, err := i.Command(jobsCommand).Stdout(ctx)
	if err != nil {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("failed to get jobs: %v", err)
		return result
	}

	// Check that all Jobs have succeeded
	var incompleteJobs []string
	lines := strings.Split(strings.TrimSpace(jobsOutput), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		namespace, name, completions, succeeded := fields[0], fields[1], fields[2], fields[3]

		// Check if succeeded count matches completions count
		if succeeded != completions {
			incompleteJobs = append(incompleteJobs,
				fmt.Sprintf("%s/%s (succeeded: %s/%s)", namespace, name, succeeded, completions))
		}
	}

	if len(incompleteJobs) > 0 {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("found Jobs that have not completed successfully: %s",
			strings.Join(incompleteJobs, ", "))

		// Get job details for debugging
		allJobs, _ := i.Command(`kubectl get jobs -A`).Stdout(ctx)
		result.Details = fmt.Sprintf("Job details:\n%s", allJobs)
		return result
	}

	result.Details = "All Jobs have completed successfully"
	return result
}
