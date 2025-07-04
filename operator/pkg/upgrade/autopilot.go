package upgrade

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/artifacts"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// determineUpgradeTargets makes sure that we are listing all the nodes in the autopilot plan.
func determineUpgradeTargets(ctx context.Context, cli client.Client) (apv1b2.PlanCommandTargets, error) {
	var nodes corev1.NodeList
	if err := cli.List(ctx, &nodes); err != nil {
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

// startAutopilotUpgrade creates an autopilot plan to upgrade to version specified in spec.config.version.
func startAutopilotUpgrade(ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig, in *v1beta1.Installation, meta *ectypes.ReleaseMetadata) error {
	targets, err := determineUpgradeTargets(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed to determine upgrade targets: %w", err)
	}

	var k0surl string
	if in.Spec.AirGap {
		// if we are running in an airgap environment all assets are already present in the
		// node and are served by the local-artifact-mirror binary listening on localhost
		// port 50000. we just need to get autopilot to fetch the k0s binary from there.
		k0surl = fmt.Sprintf("http://127.0.0.1:%d/bin/k0s-upgrade", rc.LocalArtifactMirrorPort())
	} else {
		artifact := meta.Artifacts["k0s"]
		if strings.HasPrefix(artifact, "https://") || strings.HasPrefix(artifact, "http://") {
			// for dev and e2e tests we allow the url to be overridden
			k0surl = artifact
		} else {
			k0surl = fmt.Sprintf(
				"%s/embedded-cluster-public-files/%s",
				in.Spec.MetricsBaseURL,
				artifact,
			)
		}
	}

	plan := apv1b2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot", // this is a fixed name and should not be changed
			Annotations: map[string]string{
				artifacts.InstallationNameAnnotation: in.Name,
			},
		},
		Spec: apv1b2.PlanSpec{
			Timestamp: "now",
			ID:        uuid.New().String(),
			Commands: []apv1b2.PlanCommand{
				{
					K0sUpdate: &apv1b2.PlanCommandK0sUpdate{
						Version: meta.Versions["Kubernetes"],
						Targets: targets,
						Platforms: apv1b2.PlanPlatformResourceURLMap{
							fmt.Sprintf("%s-%s", helpers.ClusterOS(), helpers.ClusterArch()): {URL: k0surl, Sha256: meta.K0sSHA},
						},
					},
				},
			},
		},
	}
	if err := cli.Create(ctx, &plan); err != nil {
		return fmt.Errorf("failed to create upgrade plan: %w", err)
	}
	in.Status.SetState(v1beta1.InstallationStateEnqueued, "", nil)
	return nil
}
