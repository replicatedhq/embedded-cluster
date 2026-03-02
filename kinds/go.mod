module github.com/replicatedhq/embedded-cluster/kinds

go 1.25.7

require (
	github.com/google/uuid v1.6.0
	github.com/k0sproject/dig v0.4.0
	github.com/k0sproject/k0s v1.33.9-0.20260218131128-cd041608f44a
	github.com/stretchr/testify v1.11.1
	go.yaml.in/yaml/v3 v3.0.4
	k8s.io/api v0.35.1
	k8s.io/apimachinery v0.35.1
	sigs.k8s.io/controller-runtime v0.22.4
	sigs.k8s.io/yaml v1.6.0
)

require (
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20240806141605-e8a1dd7889d6 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/google/pprof v0.0.0-20250820193118-f64d9cf942d6 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/vishvananda/netlink v1.3.1 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	helm.sh/helm/v3 v3.20.0 // indirect
	k8s.io/apiextensions-apiserver v0.35.1 // indirect
	k8s.io/client-go v0.35.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250910181357-589584f1c912 // indirect
	k8s.io/utils v0.0.0-20251002143259-bc988d571ff4 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
)

replace (
	// k8s staging repos: replace directives don't propagate from k0s to consumers,
	// so we must mirror k0s's replace directives here to ensure all k8s packages
	// are at a consistent version. Keep in sync with k0s go.mod when upgrading k0s.
	k8s.io/api => k8s.io/api v0.34.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.34.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.34.4
	k8s.io/apiserver => k8s.io/apiserver v0.34.4
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.34.4
	k8s.io/client-go => k8s.io/client-go v0.34.4
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.34.4
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.34.4
	k8s.io/code-generator => k8s.io/code-generator v0.34.4
	k8s.io/component-base => k8s.io/component-base v0.34.4
	k8s.io/component-helpers => k8s.io/component-helpers v0.34.4
	k8s.io/controller-manager => k8s.io/controller-manager v0.34.4
	k8s.io/cri-api => k8s.io/cri-api v0.34.4
	k8s.io/cri-client => k8s.io/cri-client v0.34.4
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.34.4
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.34.4
	k8s.io/endpointslice => k8s.io/endpointslice v0.34.4
	k8s.io/externaljwt => k8s.io/externaljwt v0.34.4
	k8s.io/kms => k8s.io/kms v0.34.4
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.34.4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.34.4
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.34.4
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.34.4
	k8s.io/kubectl => k8s.io/kubectl v0.34.4
	k8s.io/kubelet => k8s.io/kubelet v0.34.4
	k8s.io/metrics => k8s.io/metrics v0.34.4
	k8s.io/mount-utils => k8s.io/mount-utils v0.34.4
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.34.4
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.34.4
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.34.4
	k8s.io/sample-controller => k8s.io/sample-controller v0.34.4
)
