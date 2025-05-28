package controllers

import (
	"context"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileHostCABundle ensures that the CA configmap is present and is up-to-date
// with the CA bundle from the host.
func (r *InstallationReconciler) ReconcileHostCABundle(ctx context.Context) error {
	caPathInContainer := os.Getenv("PRIVATE_CA_BUNDLE_PATH")
	if caPathInContainer == "" {
		return nil
	}

	logger := ctrl.LoggerFrom(ctx)
	return kotsadm.EnsureCAConfigmap(ctx, logger.Info, r.Client, caPathInContainer, 2)
}
