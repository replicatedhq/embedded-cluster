package charts

import (
	"context"
	"k8s.io/utils/ptr"
	"testing"

	k0shelmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	registryAddon "github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	pkgrelease "github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInstallationReconciler_ReconcileHelmCharts(t *testing.T) {
	const openebsValues = `engines:
  local:
    lvm:
      enabled: false
    zfs:
      enabled: false
  replicated:
    mayastor:
      enabled: false
localpv-provisioner:
  analytics:
    enabled: false
  helperPod:
    image:
      registry: proxy.replicated.com/anonymous/
      repository: ""
      tag: ""
  hostpathClass:
    enabled: true
    isDefaultClass: true
  localpv:
    basePath: /var/lib/embedded-cluster/openebs-local
    image:
      registry: proxy.replicated.com/anonymous/
      repository: ""
      tag: ""
lvm-localpv:
  enabled: false
mayastor:
  enabled: false
preUpgradeHook:
  image:
    registry: proxy.replicated.com/anonymous
    repo: ""
    tag: ""
zfs-localpv:
  enabled: false
`

	const operatorValues = `embeddedBinaryName: test-binary-name
embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f3ed231b
embeddedClusterK0sVersion: 0.0.0
embeddedClusterVersion: 1.2.3-operator
global:
  labels:
    replicated.com/disaster-recovery: infra
    replicated.com/disaster-recovery-chart: embedded-cluster-operator
image:
  repository: docker.io/replicated/embedded-cluster-operator-image
  tag: latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8
kotsVersion: 1.2.3-admin-console
utilsImage: abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0
`
	const onlineAdminConsoleValues = `embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f3ed231b
embeddedClusterVersion: 1.2.3-operator
images:
  kotsadm: ':'
  kurlProxy: ':'
  migrations: ':'
  rqlite: ':'
isAirgap: "false"
isHA: false
isHelmManaged: false
kurlProxy:
  enabled: true
  nodePort: 30000
labels:
  replicated.com/disaster-recovery: infra
  replicated.com/disaster-recovery-chart: admin-console
minimalRBAC: false
passwordSecretRef:
  key: passwordBcrypt
  name: kotsadm-password
privateCAs:
  configmapName: kotsadm-private-cas
  enabled: true
service:
  enabled: false
`

	const airgapAdminConsoleValues = `embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f3ed231b
embeddedClusterVersion: 1.2.3-operator
images:
  kotsadm: ':'
  kurlProxy: ':'
  migrations: ':'
  rqlite: ':'
isAirgap: "true"
isHA: false
isHelmManaged: false
kurlProxy:
  enabled: true
  nodePort: 30000
labels:
  replicated.com/disaster-recovery: infra
  replicated.com/disaster-recovery-chart: admin-console
minimalRBAC: false
passwordSecretRef:
  key: passwordBcrypt
  name: kotsadm-password
privateCAs:
  configmapName: kotsadm-private-cas
  enabled: true
service:
  enabled: false
`

	const airgapHAAdminConsoleValues = `embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f3ed231b
embeddedClusterVersion: 1.2.3-operator
images:
  kotsadm: ':'
  kurlProxy: ':'
  migrations: ':'
  rqlite: ':'
isAirgap: "true"
isHA: true
isHelmManaged: false
kurlProxy:
  enabled: true
  nodePort: 30000
labels:
  replicated.com/disaster-recovery: infra
  replicated.com/disaster-recovery-chart: admin-console
minimalRBAC: false
passwordSecretRef:
  key: passwordBcrypt
  name: kotsadm-password
privateCAs:
  configmapName: kotsadm-private-cas
  enabled: true
service:
  enabled: false
`

	const veleroValues = `backupsEnabled: false
configMaps:
  fs-restore-action-config:
    data:
      image: ':'
    labels:
      velero.io/plugin-config: ""
      velero.io/pod-volume-restore: RestoreItemAction
credentials:
  existingSecret: cloud-credentials
deployNodeAgent: true
image:
  repository: ""
  tag: ""
initContainers:
- image: ':'
  imagePullPolicy: IfNotPresent
  name: velero-plugin-for-aws
  volumeMounts:
  - mountPath: /target
    name: plugins
kubectl:
  image:
    repository: ""
    tag: ""
nodeAgent:
  podVolumePath: /var/lib/embedded-cluster/k0s/kubelet/pods
snapshotsEnabled: false
`

	const registryValues = `configData:
  auth:
    htpasswd:
      path: /auth/htpasswd
      realm: Registry
extraVolumeMounts:
- mountPath: /auth
  name: auth
extraVolumes:
- name: auth
  secret:
    secretName: registry-auth
fullnameOverride: registry
image:
  repository: ""
  tag: ""
persistence:
  accessMode: ReadWriteOnce
  enabled: true
  size: 10Gi
  storageClass: openebs-hostpath
podAnnotations:
  backup.velero.io/backup-volumes: data
replicaCount: 1
service:
  clusterIP: 10.96.0.11
storage: filesystem
tlsSecretName: registry-tls
`

	const haRegistryValues = `affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchExpressions:
        - key: app
          operator: In
          values:
          - docker-registry
      topologyKey: kubernetes.io/hostname
configData:
  auth:
    htpasswd:
      path: /auth/htpasswd
      realm: Registry
  storage:
    s3:
      secure: false
extraVolumeMounts:
- mountPath: /auth
  name: auth
extraVolumes:
- name: auth
  secret:
    secretName: registry-auth
fullnameOverride: registry
image:
  repository: ""
  tag: ""
replicaCount: 2
s3:
  bucket: registry
  encrypt: false
  region: us-east-1
  regionEndpoint: DYNAMIC
  rootdirectory: /registry
  secure: false
secrets:
  s3:
    secretRef: seaweedfs-s3-rw
service:
  clusterIP: 10.96.0.11
storage: s3
tlsSecretName: registry-tls
`

	const seaweedfsValues = `filer:
  data:
    size: 1Gi
    storageClass: openebs-hostpath
    type: persistentVolumeClaim
  imageOverride: ':'
  logs:
    size: 1Gi
    storageClass: openebs-hostpath
    type: persistentVolumeClaim
  podAnnotations:
    backup.velero.io/backup-volumes: data-filer,seaweedfs-filer-log-volume
  replicas: 3
  s3:
    createBuckets:
    - anonymousRead: false
      name: registry
    enableAuth: true
    enabled: true
    existingConfigSecret: secret-seaweedfs-s3
global:
  data:
    hostPathPrefix: /var/lib/embedded-cluster/seaweedfs/ssd
  enableReplication: true
  logs:
    hostPathPrefix: /var/lib/embedded-cluster/seaweedfs/storage
  registry: proxy.replicated.com/anonymous/
  replicationPlacment: "001"
master:
  config: |-
    [master.maintenance]
    # periodically run these scripts are the same as running them from 'weed shell'
    # note: running 'fs.meta.save' then 'fs.meta.load' will ensure metadata of all filers
    # are in sync in case of data loss from 1 or more filers
    scripts = """
      ec.encode -fullPercent=95 -quietFor=1h
      ec.rebuild -force
      ec.balance -force
      volume.balance -force
      volume.configure.replication -replication 001 -collectionPattern *
      volume.fix.replication
      fs.meta.save -o filer-backup.meta
      fs.meta.load filer-backup.meta
    """
    sleep_minutes = 17          # sleep minutes between each script execution
  data:
    hostPathPrefix: /var/lib/embedded-cluster/seaweedfs/ssd
  disableHttp: true
  imageOverride: ':'
  logs:
    hostPathPrefix: /var/lib/embedded-cluster/seaweedfs/storage
  replicas: 1
  volumeSizeLimitMB: 30000
volume:
  affinity: |
    # schedule on control-plane nodes
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node-role.kubernetes.io/control-plane
            operator: Exists
    # schedule on different nodes when possible
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        - labelSelector:
            matchExpressions:
            - key: app.kubernetes.io/name
              operator: In
              values:
              - seaweedfs
            - key: app.kubernetes.io/component
              operator: In
              values:
              - volume
          topologyKey: "kubernetes.io/hostname"
  dataDirs:
  - maxVolumes: 50
    name: data
    size: 10Gi
    storageClass: openebs-hostpath
    type: persistentVolumeClaim
  imageOverride: ':'
  podAnnotations:
    backup.velero.io/backup-volumes: data
  replicas: 3
`

	var addonMetadata = map[string]pkgrelease.AddonMetadata{}

	// this function is used to replace the values of the addons so that we can test without having to update tests constantly
	replaceAddonMeta := func() {
		addonMetadata["admin-console"] = adminconsole.Metadata
		adminconsole.Metadata = pkgrelease.AddonMetadata{
			Version:  "1.2.3-admin-console",
			Location: "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
		}

		addonMetadata["embedded-cluster-operator"] = embeddedclusteroperator.Metadata
		embeddedclusteroperator.Metadata = pkgrelease.AddonMetadata{
			Location: "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
		}
		versions.Version = "1.2.3-operator" // This is not great, the operator addon uses this to determine what version to deploy
		// we can't use the version from the metadata because it won't be set in the operator binary
		// TODO fix this

		addonMetadata["openebs"] = openebs.Metadata
		openebs.Metadata = pkgrelease.AddonMetadata{
			Version:  "1.2.3-openebs",
			Location: "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
		}

		addonMetadata["registry"] = registryAddon.Metadata
		registryAddon.Metadata = pkgrelease.AddonMetadata{
			Version:  "1.2.3-registry",
			Location: "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/docker-registry",
		}

		addonMetadata["seaweedfs"] = seaweedfs.Metadata
		seaweedfs.Metadata = pkgrelease.AddonMetadata{
			Version:  "1.2.3-seaweedfs",
			Location: "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/seaweedfs",
		}

		addonMetadata["velero"] = velero.Metadata
		velero.Metadata = pkgrelease.AddonMetadata{
			Version:  "1.2.3-velero",
			Location: "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/velero",
		}

		adminconsole.Render()
		embeddedclusteroperator.Render()
		openebs.Render()
		registryAddon.Render()
		seaweedfs.Render()
		velero.Render()
	}

	restoreAddonMeta := func() {
		adminconsole.Metadata = addonMetadata["admin-console"]
		embeddedclusteroperator.Metadata = addonMetadata["embedded-cluster-operator"]
		openebs.Metadata = addonMetadata["openebs"]
		registryAddon.Metadata = addonMetadata["registry"]
		seaweedfs.Metadata = addonMetadata["seaweedfs"]
		velero.Metadata = addonMetadata["velero"]

		adminconsole.Render()
		embeddedclusteroperator.Render()
		openebs.Render()
		registryAddon.Render()
		seaweedfs.Render()
		velero.Render()
	}

	replaceAddonMeta()
	defer restoreAddonMeta()

	type fields struct {
		State     []runtime.Object
		Discovery discovery.DiscoveryInterface
		Scheme    *runtime.Scheme
	}
	tests := []struct {
		name        string
		fields      fields
		in          v1beta1.Installation
		out         v1beta1.InstallationStatus
		releaseMeta ectypes.ReleaseMetadata
		updatedHelm *k0sv1beta1.HelmExtensions
	}{
		{
			name: "no input config, move to installed",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
			},
			out: v1beta1.InstallationStatus{State: v1beta1.InstallationStateInstalled, Reason: "Installed"},
		},
		{
			name: "k8s install in progress, no state change",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateInstalling},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "abc",
					},
				},
			},
			out: v1beta1.InstallationStatus{State: v1beta1.InstallationStateInstalling},
		},
		{
			name: "k8s install completed, good version, both types of charts, no drift",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &v1beta1.Helm{
								Charts: []v1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
					ClusterID:  "e79f0701-67f3-4abf-a672-42a1f3ed231b",
					BinaryName: "test-binary-name",
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateInstalled,
				Reason: "Addons upgraded",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Images: []string{
					"abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0",
					"docker.io/replicated/another-image:latest-arm64@sha256:a9ab9db181f9898283a87be0f79d85cb8f3d22a790b71f52c8a9d339e225dedd",
					"docker.io/replicated/embedded-cluster-operator-image:latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8",
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "extchart",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "extchart"},
						Status: k0shelmv1beta1.ChartStatus{Version: "2", ValuesHash: "c687e5ae3f4a71927fb7ba3a3fee85f40c2debeec3b8bf66d038955a60ccf3ba"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "openebs",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "openebs", Values: openebsValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-openebs", ValuesHash: "c0ea0af176f78196117571c1a50f6f679da08a2975e442fe39542cbe419b55c6"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "embedded-cluster-operator",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "embedded-cluster-operator", Values: operatorValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-operator", ValuesHash: "215c33c6a56953b6d6814251f6fa0e78d3884a4d15dbb515a3942baf40900893"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin-console",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "admin-console", Values: onlineAdminConsoleValues},
						Status: k0shelmv1beta1.ChartStatus{Version: "1.2.3-admin-console", ValuesHash: "88e04728e85bbbf8a7c676a28c6bc7809273c8a0aa21ed0a407c635855b6944e"},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "extchart",
											Version: "2",
										},
										{
											Name:         "openebs",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
											Version:      "1.2.3-openebs",
											Values:       openebsValues,
											TargetNS:     "openebs",
											ForceUpgrade: ptr.To(false),
											Order:        101,
										},
										{
											Name:         "embedded-cluster-operator",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
											Version:      "1.2.3-operator",
											Values:       operatorValues,
											TargetNS:     "embedded-cluster",
											ForceUpgrade: ptr.To(false),
											Order:        103,
										},
										{
											Name:         "admin-console",
											ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
											Version:      "1.2.3-admin-console",
											Values:       onlineAdminConsoleValues,
											TargetNS:     "kotsadm",
											ForceUpgrade: ptr.To(false),
											Order:        105,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, good version, both types of charts, chart errors",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &v1beta1.Helm{
								Charts: []v1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateHelmChartUpdateFailure,
				Reason: "failed to update helm charts: \nextchart: exterror\n",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: v1beta1.Helm{
					Charts: []v1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{ReleaseName: "metachart"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "extchart",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "extchart"},
						Status: k0shelmv1beta1.ChartStatus{Version: "2", Error: "exterror"},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
										},
										{
											Name:    "extchart",
											Version: "2",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, good version, releaseMeta chart, chart errors",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateHelmChartUpdateFailure,
				Reason: "failed to update helm charts: \nmetachart: metaerror\n",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: v1beta1.Helm{
					Charts: []v1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "metachart"},
						Status: k0shelmv1beta1.ChartStatus{Version: "1", Error: "metaerror"},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, good version, both types of charts, drift, addons already installing",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateAddonsInstalling},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &v1beta1.Helm{
								Charts: []v1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State: v1beta1.InstallationStateAddonsInstalling,
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: v1beta1.Helm{
					Charts: []v1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, good version, both types of charts, drift",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &v1beta1.Helm{
								Charts: []v1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateAddonsInstalling,
				Reason: "Installing addons",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: v1beta1.Helm{
					Charts: []v1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Order:   1,
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{},
							},
						},
					},
				},
			},
			updatedHelm: &k0sv1beta1.HelmExtensions{
				Charts: []k0sv1beta1.Chart{
					{
						Name:    "metachart",
						Version: "1",
						Order:   101,
					},
					{
						Name:    "extchart",
						Version: "2",
						Order:   110,
					},
				},
			},
		},
		{
			name: "k8s install completed, good version, admin console and operator values, both types of charts, no drift",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					ClusterID:        "test cluster ID",
					BinaryName:       "test-binary-name",
					AirGap:           false,
					HighAvailability: false,
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &v1beta1.Helm{
								Charts: []v1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateInstalled,
				Reason: "Addons upgraded",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: v1beta1.Helm{
					Charts: []v1beta1.Chart{
						{
							Name:    "admin-console",
							Version: "1",
							Values: `
abc: xyz
password: frommeta`,
						},
						{
							Name:    "embedded-cluster-operator",
							Version: "1",
							Values: `
abc: xyz
password: frommeta`,
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin-console",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "admin-console",
							Values: `abc: xyz
embeddedClusterID: test cluster ID
isAirgap: "false"
isHA: false
password: frommeta`,
						},
						Status: k0shelmv1beta1.ChartStatus{Version: "1", ValuesHash: "a785ac98c2dc3e962fa3bf0e38c4d42f2380a204f1fc1a4a30cfe8732750fb9e"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "embedded-cluster-operator",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "embedded-cluster-operator",
							Values: `abc: xyz
embeddedBinaryName: test-binary-name
embeddedClusterID: test cluster ID
password: frommeta`,
						},
						Status: k0shelmv1beta1.ChartStatus{Version: "1", ValuesHash: "2b3f4301ee3da37c75b573e12fc8e0cb0dc81ec1fbf5a1084b9adc198f06bbb0"},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "extchart",
						},
						Spec:   k0shelmv1beta1.ChartSpec{ReleaseName: "extchart"},
						Status: k0shelmv1beta1.ChartStatus{Version: "2", ValuesHash: "c687e5ae3f4a71927fb7ba3a3fee85f40c2debeec3b8bf66d038955a60ccf3ba"},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "admin-console",
											Version: "1",
											Values: `
abc: xyz
embeddedClusterID: test cluster ID
isAirgap: "false"
isHA: false
password: frommeta`,
										},
										{
											Name:    "embedded-cluster-operator",
											Version: "1",
											Values: `
abc: xyz
embeddedBinaryName: test-binary-name
embeddedClusterID: test cluster ID
password: frommeta`,
										},
										{
											Name:    "extchart",
											Version: "2",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, good version, overridden values, both types of charts, values drift",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
						Extensions: v1beta1.Extensions{
							Helm: &v1beta1.Helm{
								Charts: []v1beta1.Chart{
									{
										Name:    "extchart",
										Version: "2",
									},
								},
							},
						},
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateAddonsInstalling,
				Reason: "Installing addons",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: v1beta1.Helm{
					Charts: []v1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Values: `
abc: xyz
password: overridden`,
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "metachart",
							Values: `abc: original
password: original`,
						},
						Status: k0shelmv1beta1.ChartStatus{
							Version:     "1",
							ReleaseName: "metachart",
							ValuesHash:  "1fcf324bc7890a68f7402a7a523bb47a470b726f1011f69c3d7cf2e911f15685",
						},
					},
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "extchart",
						},
						Spec: k0shelmv1beta1.ChartSpec{ReleaseName: "extchart"},
						Status: k0shelmv1beta1.ChartStatus{
							Version:     "2",
							ReleaseName: "extchart",
							ValuesHash:  "c687e5ae3f4a71927fb7ba3a3fee85f40c2debeec3b8bf66d038955a60ccf3ba",
						},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
											Values: `
abc: original
password: original`,
										},
										{
											Name:    "extchart",
											Version: "2",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, values drift but chart not yet installed",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:         v1beta1.InstallationStatePendingChartCreation,
				Reason:        "Pending charts: [metachart]",
				PendingCharts: []string{"metachart"},
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: v1beta1.Helm{
					Charts: []v1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Values:  `abc: xyz`,
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "metachart",
							Values:      `abc: original`,
						},
						Status: k0shelmv1beta1.ChartStatus{},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
											Values:  `abc: original`,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, no values drift but chart not yet installed",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:         v1beta1.InstallationStatePendingChartCreation,
				Reason:        "Pending charts: [metachart]",
				PendingCharts: []string{"metachart"},
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: v1beta1.Helm{
					Charts: []v1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Values:  `abc: xyz`,
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "metachart",
							Version:     "1",
							Values:      `abc: original`,
						},
						Status: k0shelmv1beta1.ChartStatus{
							ReleaseName: "metachart",
						},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
											Values:  `abc: xyz`,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, updating charts despite errors",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateAddonsInstalling,
				Reason: "Installing addons",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: v1beta1.Helm{
					Charts: []v1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Values:  `abc: xyz`,
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "metachart",
							Version:     "1",
							Values:      `abc: original`,
						},
						Status: k0shelmv1beta1.ChartStatus{
							ReleaseName: "metachart",
							Error:       "error",
						},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
											Values:  `abc: original`,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "k8s install completed, no values drift",
			in: v1beta1.Installation{
				Status: v1beta1.InstallationStatus{State: v1beta1.InstallationStateKubernetesInstalled},
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version: "goodver",
					},
				},
			},
			out: v1beta1.InstallationStatus{
				State:  v1beta1.InstallationStateInstalled,
				Reason: "Addons upgraded",
			},
			releaseMeta: ectypes.ReleaseMetadata{
				Configs: v1beta1.Helm{
					Charts: []v1beta1.Chart{
						{
							Name:    "metachart",
							Version: "1",
							Values:  `abc: xyz`,
						},
					},
				},
			},
			fields: fields{
				State: []runtime.Object{
					&k0shelmv1beta1.Chart{
						ObjectMeta: metav1.ObjectMeta{
							Name: "metachart",
						},
						Spec: k0shelmv1beta1.ChartSpec{
							ReleaseName: "metachart",
							Version:     "1",
							Values:      `abc: xyz`,
						},
						Status: k0shelmv1beta1.ChartStatus{
							ReleaseName: "metachart",
							Version:     "1",
							ValuesHash:  "dace29a7a92865fa8a5dcd85540a806aa9cf0a7d37fa119f2546a17afd7e33b4",
						},
					},
					&k0sv1beta1.ClusterConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "k0s",
							Namespace: "kube-system",
						},
						Spec: &k0sv1beta1.ClusterSpec{
							Extensions: &k0sv1beta1.ClusterExtensions{
								Helm: &k0sv1beta1.HelmExtensions{
									Charts: []k0sv1beta1.Chart{
										{
											Name:    "metachart",
											Version: "1",
											Values:  `abc: xyz`,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			release.CacheMeta("goodver", tt.releaseMeta)

			sch := runtime.NewScheme()
			req.NoError(k0sv1beta1.AddToScheme(sch))
			req.NoError(k0shelmv1beta1.AddToScheme(sch))
			fakeCli := fake.NewClientBuilder().WithScheme(sch).WithRuntimeObjects(tt.fields.State...).Build()

			_, err := ReconcileHelmCharts(context.Background(), fakeCli, &tt.in)
			req.NoError(err)
			req.Equal(tt.out, tt.in.Status)

			if tt.updatedHelm != nil {
				var gotCluster k0sv1beta1.ClusterConfig
				err = fakeCli.Get(context.Background(), client.ObjectKey{Name: "k0s", Namespace: "kube-system"}, &gotCluster)
				req.NoError(err)
				req.ElementsMatch(tt.updatedHelm.Charts, gotCluster.Spec.Extensions.Helm.Charts)
				req.ElementsMatch(tt.updatedHelm.Repositories, gotCluster.Spec.Extensions.Helm.Repositories)
			}
		})
	}
}
