package preflights

import (
	"context"
	"fmt"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

type PrepareAndRunOptions struct {
	License *kotsv1beta1.License
	Proxy   *ecv1beta1.ProxySpec

	ConnectivityFromCIDR string
	ConnectivityToCIDR   string
}

func PrepareAndRun(ctx context.Context, opts PrepareAndRunOptions) error {
	replicatedAPIURL := opts.License.Spec.Endpoint
	proxyRegistryURL := fmt.Sprintf("https://%s", opts.Proxy.HTTPSProxy)

	fmt.Printf("Running host preflights: %s, %s\n", replicatedAPIURL, proxyRegistryURL)
	return nil
}
