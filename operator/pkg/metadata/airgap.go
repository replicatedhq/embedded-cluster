package metadata

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/artifacts"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CopyVersionMetadataToCluster makes sure a config map with the embedded cluster version metadata exists in the
// cluster. The data is read from the internal registry on the repository pointed by EmbeddedClusterMetadata.
func CopyVersionMetadataToCluster(ctx context.Context, cli client.Client, in *v1beta1.Installation) error {
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
	if err := cli.Get(ctx, nsn, &cm); err == nil {
		return nil
	} else if !k8serrors.IsNotFound(err) {
		return fmt.Errorf("get configmap: %w", err)
	}

	// pull the artifact from the artifact location pointed by EmbeddedClusterMetadata. This property
	// points to a repository inside the registry running on the cluster.
	location, err := artifacts.Pull(ctx, log, cli, in.Spec.Artifacts.EmbeddedClusterMetadata)
	if err != nil {
		return fmt.Errorf("pull artifact: %w", err)
	}
	defer os.RemoveAll(location)

	// now that we have the metadata locally we can read its information and create the config map.
	fpath := filepath.Join(location, "version-metadata.json")
	data, err := os.ReadFile(fpath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	cm.Name = nsn.Name
	cm.Namespace = nsn.Namespace
	cm.Data = map[string]string{"metadata.json": string(data)}
	if err := cli.Create(ctx, &cm); err != nil {
		return fmt.Errorf("create configmap: %w", err)
	}
	return nil
}
