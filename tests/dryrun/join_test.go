package dryrun

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
)

func TestJoin(t *testing.T) {
	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	client := &dryrun.Client{
		Kotsadm: dryrun.NewKotsadm(),
	}
	clusterID := uuid.New()
	jcmd := &kotsadm.JoinCommandResponse{
		K0sJoinCommand:         "/usr/local/bin/k0s install controller --enable-worker --no-taints --labels kots.io/embedded-cluster-role=total-1,kots.io/embedded-cluster-role-0=controller-test,controller-label=controller-label-value",
		K0sToken:               "some-k0s-token",
		EmbeddedClusterVersion: "v0.0.0",
		ClusterID:              clusterID,
		InstallationSpec: ecv1beta1.InstallationSpec{
			ClusterID: clusterID.String(),
			Config: &ecv1beta1.ConfigSpec{
				UnsupportedOverrides: ecv1beta1.UnsupportedOverrides{},
			},
		},
	}
	client.Kotsadm.SetGetJoinTokenResponse("10.0.0.1", "some-token", jcmd, nil)
	dryrun.Init(drFile, client)
	dryrunJoin(t, "10.0.0.1", "some-token")
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
