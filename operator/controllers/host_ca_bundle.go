package controllers

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	err := adminconsole.EnsureCAConfigmap(ctx, logger.Info, r.Client, caPathInContainer)
	if k8serrors.IsRequestEntityTooLargeError(err) || errors.Is(err, fs.ErrNotExist) {
		logger.Error(err, "Failed to reconcile host ca bundle")
		return nil
	}
	return err
}
