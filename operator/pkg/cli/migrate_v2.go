package cli

import (
	"context"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func runMigrateV2(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) error {
	return nil
}
