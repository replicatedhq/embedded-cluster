package highavailability

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"

	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
)

const (
	// seaweedfsS3SVCName is the name of the Seaweedfs S3 service managed by the operator.
	// HACK: This service has a hardcoded service IP shared by the cli and operator as it is used
	// by the registry to redirect requests for blobs.
	seaweedfsS3SVCName = "ec-seaweedfs-s3"

	// seaweedfsLowerBandIPIndex is the index of the seaweedfs service IP in the service CIDR.
	// HACK: this is shared with the cli and operator as it is used by the registry to redirect requests for blobs.
	seaweedfsLowerBandIPIndex = 11

	// seaweedfsS3SecretName is the name of the Seaweedfs secret.
	// This secret name is defined in the chart in the release metadata.
	seaweedfsS3SecretName = "secret-seaweedfs-s3"
)

type seaweedfsConfig struct {
	Identities []seaweedfsIdentity `json:"identities"`
}

type seaweedfsIdentity struct {
	Name        string                        `json:"name"`
	Credentials []seaweedfsIdentityCredential `json:"credentials"`
	Actions     []string                      `json:"actions"`
}

type seaweedfsIdentityCredential struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

func (c seaweedfsConfig) getCredentials(name string) (seaweedfsIdentityCredential, bool) {
	for _, identity := range c.Identities {
		if identity.Name == name {
			if len(identity.Credentials) == 0 {
				return seaweedfsIdentityCredential{}, false
			}
			return identity.Credentials[0], true
		}
	}
	return seaweedfsIdentityCredential{}, false
}

func createSeaweedfsResources(ctx context.Context, kcli client.Client, in *embeddedclusterv1beta1.Installation) (*seaweedfsConfig, error) {
	err := ensureSeaweedfsNamespace(ctx, kcli)
	if err != nil {
		return nil, fmt.Errorf("unable to create seaweedfs namespace: %w", err)
	}

	err = kubeutils.WaitForNamespace(ctx, kcli, defaults.SeaweedFSNamespace)
	if err != nil {
		return nil, fmt.Errorf("wait for seaweedfs namespace: %w", err)
	}

	err = ensureSeaweedfsS3Service(ctx, in, kcli)
	if err != nil {
		return nil, fmt.Errorf("unable to create seaweedfs s3 service: %w", err)
	}

	seaweedConfig, err := ensureSeaweedFSS3Secret(ctx, kcli)
	if err != nil {
		return nil, fmt.Errorf("unable to create seaweedfs s3 secret: %w", err)
	}

	return seaweedConfig, nil
}

func ensureSeaweedfsNamespace(ctx context.Context, cli client.Client) error {
	obj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.SeaweedFSNamespace},
	}

	err := cli.Create(ctx, obj)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create seaweedfs namespace: %w", err)

	}

	return nil
}

func ensureSeaweedfsS3Service(ctx context.Context, in *embeddedclusterv1beta1.Installation, cli client.Client) error {
	err := ensureSeaweedfsNamespace(ctx, cli)
	if err != nil {
		return fmt.Errorf("ensure seaweedfs namespace: %w", err)
	}

	if in == nil || in.Spec.Network == nil || in.Spec.Network.ServiceCIDR == "" {
		return fmt.Errorf("installation spec network or service not present")
	}

	clusterIP, err := getSeaweedfsS3ServiceIP(in.Spec.Network.ServiceCIDR)
	if err != nil {
		return fmt.Errorf("get seaweedfs s3 service IP: %w", err)
	}

	obj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: seaweedfsS3SVCName, Namespace: defaults.SeaweedFSNamespace},
		Spec: corev1.ServiceSpec{
			ClusterIP: clusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "swfs-s3",
					Port:       8333,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8333),
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/component": "filer",
				"app.kubernetes.io/name":      "seaweedfs",
			},
		},
	}
	obj.ObjectMeta.Labels = applySeaweedFSLabels(obj.ObjectMeta.Labels, "s3")

	err = cli.Create(ctx, obj)
	if err != nil {
		return fmt.Errorf("create registry seaweedfs s3 service: %w", err)
	}

	return nil
}

func ensureSeaweedFSS3Secret(ctx context.Context, cli client.Client) (*seaweedfsConfig, error) {
	var config seaweedfsConfig
	config.Identities = append(config.Identities, seaweedfsIdentity{
		Name: "anvAdmin",
		Credentials: []seaweedfsIdentityCredential{{
			AccessKey: helpers.RandString(20),
			SecretKey: helpers.RandString(40),
		}},
		Actions: []string{"Admin", "Read", "Write"},
	})
	config.Identities = append(config.Identities, seaweedfsIdentity{
		Name: "anvReadOnly",
		Credentials: []seaweedfsIdentityCredential{{
			AccessKey: helpers.RandString(20),
			SecretKey: helpers.RandString(40),
		}},
		Actions: []string{"Read"},
	})

	configData, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal seaweedfs_s3_config: %w", err)
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: seaweedfsS3SecretName, Namespace: defaults.SeaweedFSNamespace},
		Data: map[string][]byte{
			"seaweedfs_s3_config": configData,
		},
	}

	obj.ObjectMeta.Labels = applySeaweedFSLabels(obj.ObjectMeta.Labels, "s3")

	err = cli.Create(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("create registry seaweedfs s3 service: %w", err)
	}
	return &config, nil
}

func applySeaweedFSLabels(labels map[string]string, component string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["app.kubernetes.io/name"] = "seaweedfs" // this is the backup/restore label for seaweedfs
	labels["app.kubernetes.io/component"] = component
	labels["app.kubernetes.io/instance"] = "seaweedfs"
	labels["app.kubernetes.io/part-of"] = "embedded-cluster"
	labels["app.kubernetes.io/managed-by"] = "embedded-cluster-operator"
	return labels
}

func getSeaweedfsS3ServiceIP(serviceCIDR string) (string, error) {
	ip, err := util.GetLowerBandIP(serviceCIDR, seaweedfsLowerBandIPIndex)
	if err != nil {
		return "", fmt.Errorf("get lower band ip at index %d: %w", seaweedfsLowerBandIPIndex, err)
	}
	return ip.String(), nil
}
