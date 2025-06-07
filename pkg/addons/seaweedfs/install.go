package seaweedfs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *SeaweedFS) Install(
	ctx context.Context, clients types.Clients, writer *spinner.MessageWriter,
	inSpec ecv1beta1.InstallationSpec, overrides []string, installOpts types.InstallOptions,
) error {
	if err := s.ensurePreRequisites(ctx, clients, inSpec); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := s.GenerateHelmValues(ctx, inSpec, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	err = s.ensurePostInstallHooksDeleted(ctx, clients)
	if err != nil {
		return errors.Wrap(err, "ensure hooks deleted")
	}

	_, err = clients.HelmClient.Install(ctx, helm.InstallOptions{
		ReleaseName:  releaseName,
		ChartPath:    s.ChartLocation(runtimeconfig.GetDomains(inSpec.Config)),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    s.Namespace(),
		Labels:       getBackupLabels(),
	})
	if err != nil {
		return errors.Wrap(err, "helm install")
	}
	return nil
}

func (s *SeaweedFS) ensurePreRequisites(ctx context.Context, clients types.Clients, inSpec ecv1beta1.InstallationSpec) error {
	if err := s.ensureNamespace(ctx, clients); err != nil {
		return errors.Wrap(err, "create namespace")
	}

	if err := s.ensureService(ctx, clients, inSpec); err != nil {
		return errors.Wrap(err, "create s3 service")
	}

	if err := s.ensureS3Secret(ctx, clients); err != nil {
		return errors.Wrap(err, "create s3 secret")
	}

	return nil
}

func (s *SeaweedFS) ensureNamespace(ctx context.Context, clients types.Clients) error {
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.Namespace(),
		},
	}
	if err := clients.K8sClient.Create(ctx, &ns); client.IgnoreAlreadyExists(err) != nil {
		return err
	}
	return nil
}

func (s *SeaweedFS) ensureService(ctx context.Context, clients types.Clients, inSpec ecv1beta1.InstallationSpec) error {
	var serviceCIDR string
	if inSpec.Network != nil && inSpec.Network.ServiceCIDR != "" {
		serviceCIDR = inSpec.Network.ServiceCIDR
	} else {
		var err error
		_, serviceCIDR, err = netutils.SplitNetworkCIDR(ecv1beta1.DefaultNetworkCIDR)
		if err != nil {
			return fmt.Errorf("split default network CIDR: %w", err)
		}
	}

	clusterIP, err := getServiceIP(serviceCIDR)
	if err != nil {
		return errors.Wrap(err, "get s3 service IP")
	}

	obj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: s3SVCName, Namespace: s.Namespace()},
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

	obj.ObjectMeta.Labels = ApplyLabels(obj.ObjectMeta.Labels, "s3")

	var existingObj corev1.Service
	if err := clients.K8sClient.Get(ctx, client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}, &existingObj); client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "get s3 service")
	} else if err == nil {
		// if the service already exists and has the correct cluster IP, do not recreate it
		if existingObj.Spec.ClusterIP == clusterIP {
			return nil
		}
		err := clients.K8sClient.Delete(ctx, &existingObj)
		if err != nil {
			return errors.Wrap(err, "delete existing s3 service")
		}
	}

	if err := clients.K8sClient.Create(ctx, obj); err != nil {
		return errors.Wrap(err, "create s3 service")
	}

	return nil
}

func (s *SeaweedFS) ensureS3Secret(ctx context.Context, clients types.Clients) error {
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
		return errors.Wrap(err, "marshal seaweedfs_s3_config")
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: s3SecretName, Namespace: s.Namespace()},
		Data: map[string][]byte{
			"seaweedfs_s3_config": configData,
		},
	}

	obj.ObjectMeta.Labels = ApplyLabels(obj.ObjectMeta.Labels, "s3")

	if err := clients.K8sClient.Create(ctx, obj); client.IgnoreAlreadyExists(err) != nil {
		return errors.Wrap(err, "create s3 secret")
	}

	return nil
}

// ensurePostInstallHooksDeleted will delete helm hooks if for some reason they fail. It is
// necessary if the hook does not have the "before-hook-creation" delete policy.
func (s *SeaweedFS) ensurePostInstallHooksDeleted(ctx context.Context, clients types.Clients) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.Namespace(),
			Name:      fmt.Sprintf("%s-bucket-hook", releaseName),
		},
	}
	err := clients.K8sClient.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground))
	if client.IgnoreNotFound(err) != nil {
		return errors.Wrapf(err, "delete %s-bucket-hook job", releaseName)
	}

	return nil
}

func ApplyLabels(labels map[string]string, component string) map[string]string {
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

func GetS3RWCreds(ctx context.Context, kcli client.Client) (string, string, error) {
	secret := &corev1.Secret{}
	err := kcli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: s3SecretName}, secret)
	if err != nil {
		return "", "", errors.Wrap(err, "get s3 secret")
	}

	configData, ok := secret.Data["seaweedfs_s3_config"]
	if !ok {
		return "", "", errors.New("seaweedfs_s3_config not found in secret")
	}

	var config seaweedfsConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return "", "", errors.Wrap(err, "unmarshal seaweedfs_s3_config")
	}

	for _, identity := range config.Identities {
		if identity.Name == "anvAdmin" && len(identity.Credentials) > 0 {
			return identity.Credentials[0].AccessKey, identity.Credentials[0].SecretKey, nil
		}
	}

	return "", "", errors.New("credentials not found")
}

func GetS3URL(serviceCIDR string) (string, error) {
	endpoint, err := GetS3Endpoint(serviceCIDR)
	if err != nil {
		return "", errors.Wrap(err, "get s3 endpoint")
	}
	return fmt.Sprintf("http://%s", endpoint), nil
}

func GetS3Endpoint(serviceCIDR string) (string, error) {
	ip, err := getServiceIP(serviceCIDR)
	if err != nil {
		return "", errors.Wrap(err, "get service IP")
	}
	return fmt.Sprintf("%s:8333", ip), nil
}

func getServiceIP(serviceCIDR string) (string, error) {
	ip, err := helpers.GetLowerBandIP(serviceCIDR, lowerBandIPIndex)
	if err != nil {
		return "", errors.Wrap(err, "get lower band ip")
	}
	return ip.String(), nil
}
