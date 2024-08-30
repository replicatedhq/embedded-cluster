package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/k0sproject/version"
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *InstallationReconciler) shouldUpgradeK0s(ctx context.Context, in *clusterv1beta1.Installation, desiredK0sVersion string) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	// if the kubernetes version has changed we create an upgrade command.
	serverVersion, err := r.Discovery.ServerVersion()
	if err != nil {
		return false, fmt.Errorf("get server version: %w", err)
	}
	runningServerVersion, err := version.NewVersion(serverVersion.GitVersion)
	if err != nil {
		return false, fmt.Errorf("parse running server version: %w", err)
	}
	desiredServerVersion, err := k8sServerVersionFromK0sVersion(desiredK0sVersion)
	if err != nil {
		return false, fmt.Errorf("parse desired server version: %w", err)
	}

	if desiredServerVersion.GreaterThan(runningServerVersion) {
		log.Info("K0s upgrade required", "desired", desiredServerVersion, "running", runningServerVersion)
		return true, nil
	} else if desiredServerVersion.LessThan(runningServerVersion) {
		log.V(5).Info("K0s downgrade not supported", "desired", desiredServerVersion, "running", runningServerVersion)
		return false, nil
	}

	// if this is the same server version we may be able to tell the actual k0s version from the
	// previous installation
	previousK0sVersion, err := r.discoverPreviousK0sVersion(ctx, in)
	if err != nil {
		return false, fmt.Errorf("discover previous k0s version: %w", err)
	}
	if previousK0sVersion != "" && desiredK0sVersion != previousK0sVersion {
		log.Info("K0s upgrade required", "desired", desiredK0sVersion, "previous", previousK0sVersion)
		return true, nil
	}
	log.V(5).Info("K0s upgrade not required", "desired", desiredK0sVersion, "previous", previousK0sVersion)
	return false, nil
}

// discoverPreviousK0sVersion gets the k0s version from the previous installation object.
func (r *InstallationReconciler) discoverPreviousK0sVersion(ctx context.Context, in *clusterv1beta1.Installation) (string, error) {
	ins, err := r.listInstallations(ctx)
	if err != nil {
		return "", fmt.Errorf("list installations: %w", err)
	}
	for _, i := range ins {
		if i.Name == in.Name {
			continue
		}
		// the previous installation should be the second one in the list
		meta, err := release.MetadataFor(ctx, &i, r.Client)
		if err != nil {
			return "", fmt.Errorf("get release metadata for installation %s: %w", i.Name, err)
		}
		if v := meta.Versions["Kubernetes"]; v != "" {
			return v, nil
		}
		return "", nil
	}
	return "", nil
}

// if we have installed the cluster with a k0s version like v1.29.1+k0s.1 then
// the kubernetes server version reported back is v1.29.1+k0s. i.e. the .1 is
// not part of the kubernetes version, it is the k0s version. we trim it down
// so we can compare kube with kube version.
func k8sServerVersionFromK0sVersion(k0sVersion string) (*version.Version, error) {
	index := strings.Index(k0sVersion, "+k0s")
	if index == -1 {
		return nil, fmt.Errorf("invalid k0s version")
	}
	k0sVersion = k0sVersion[:index+len("+k0s")]
	v, err := version.NewVersion(k0sVersion)
	if err != nil {
		return nil, fmt.Errorf("parse k0s version: %w", err)
	}
	return v, nil
}
