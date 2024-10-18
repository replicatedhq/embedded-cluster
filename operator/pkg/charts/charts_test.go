package charts

import (
	"context"
	"testing"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	registryAddon "github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const test_openebsValues = `engines:
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

const test_operatorValues = `embeddedBinaryName: test-binary-name
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

const test_overriddenOperatorValues = `embeddedBinaryName: test-binary-name
embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f3ed231b
embeddedClusterK0sVersion: 0.0.0
embeddedClusterVersion: abctest
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

const test_onlineAdminConsoleValues = `embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f3ed231b
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

const test_overriddenOnlineAdminConsoleValues = `embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f3ed231b
embeddedClusterVersion: abctest
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

const test_airgapAdminConsoleValues = `embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f3ed231b
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

const test_airgapHAAdminConsoleValues = `embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f3ed231b
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

const test_veleroValues = `backupsEnabled: false
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

const test_registryValues = `configData:
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

const test_haRegistryValues = `affinity:
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
  regionEndpoint: 10.96.0.12:8333
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

const test_seaweedfsValues = `filer:
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

var test_addonMetadata = map[string]release.AddonMetadata{}

// this function is used to replace the values of the addons so that we can run unit tests without having to update values constantly
func test_replaceAddonMeta() {
	test_addonMetadata["admin-console"] = adminconsole.Metadata
	adminconsole.Metadata = release.AddonMetadata{
		Version:  "1.2.3-admin-console",
		Location: "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
	}

	test_addonMetadata["embedded-cluster-operator"] = embeddedclusteroperator.Metadata
	embeddedclusteroperator.Metadata = release.AddonMetadata{
		Location: "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
	}
	versions.Version = "1.2.3-operator" // This is not great, the operator addon uses this to determine what version to deploy
	// we can't use the version from the metadata because it won't be set in the operator binary
	// TODO fix this

	test_addonMetadata["openebs"] = openebs.Metadata
	openebs.Metadata = release.AddonMetadata{
		Version:  "1.2.3-openebs",
		Location: "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
	}

	test_addonMetadata["registry"] = registryAddon.Metadata
	registryAddon.Metadata = release.AddonMetadata{
		Version:  "1.2.3-registry",
		Location: "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/docker-registry",
	}

	test_addonMetadata["seaweedfs"] = seaweedfs.Metadata
	seaweedfs.Metadata = release.AddonMetadata{
		Version:  "1.2.3-seaweedfs",
		Location: "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/seaweedfs",
	}

	test_addonMetadata["velero"] = velero.Metadata
	velero.Metadata = release.AddonMetadata{
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

func test_restoreAddonMeta() {
	adminconsole.Metadata = test_addonMetadata["admin-console"]
	embeddedclusteroperator.Metadata = test_addonMetadata["embedded-cluster-operator"]
	openebs.Metadata = test_addonMetadata["openebs"]
	registryAddon.Metadata = test_addonMetadata["registry"]
	seaweedfs.Metadata = test_addonMetadata["seaweedfs"]
	velero.Metadata = test_addonMetadata["velero"]

	adminconsole.Render()
	embeddedclusteroperator.Render()
	openebs.Render()
	registryAddon.Render()
	seaweedfs.Render()
	velero.Render()
}

func Test_generateHelmConfigs(t *testing.T) {
	test_replaceAddonMeta()
	defer test_restoreAddonMeta()

	type args struct {
		in            v1beta1.Extensions
		conditions    []metav1.Condition
		clusterConfig k0sv1beta1.ClusterConfig
	}
	tests := []struct {
		name             string
		args             args
		airgap           bool
		highAvailability bool
		disasterRecovery bool
		operatorLocation string
		want             *v1beta1.Helm
	}{
		{
			name:             "online non-ha no-velero",
			airgap:           false,
			highAvailability: false,
			disasterRecovery: false,
			args: args{
				in: v1beta1.Extensions{
					Helm: &v1beta1.Helm{
						ConcurrencyLevel: 2,
						Repositories:     nil,
						Charts: []v1beta1.Chart{
							{
								Name:    "test",
								Version: "1.0.0",
								Order:   20,
							},
						},
					},
				},
			},
			want: &v1beta1.Helm{
				ConcurrencyLevel: 1,
				Repositories:     nil,
				Charts: []v1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Order:   120,
					},
					{
						Name:         "openebs",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
						Version:      "1.2.3-openebs",
						Values:       test_openebsValues,
						TargetNS:     "openebs",
						ForceUpgrade: ptr.To(false),
						Order:        101,
					},
					{
						Name:         "embedded-cluster-operator",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
						Version:      "1.2.3-operator",
						Values:       test_operatorValues,
						TargetNS:     "embedded-cluster",
						ForceUpgrade: ptr.To(false),
						Order:        103,
					},
					{
						Name:         "admin-console",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
						Version:      "1.2.3-admin-console",
						Values:       test_onlineAdminConsoleValues,
						TargetNS:     "kotsadm",
						ForceUpgrade: ptr.To(false),
						Order:        105,
					},
				},
			},
		},
		{
			name:             "online non-ha no-velero different operator location",
			operatorLocation: "test-operator-location",
			airgap:           false,
			highAvailability: false,
			disasterRecovery: false,
			args: args{
				in: v1beta1.Extensions{
					Helm: &v1beta1.Helm{
						ConcurrencyLevel: 2,
						Repositories:     nil,
						Charts: []v1beta1.Chart{
							{
								Name:    "test",
								Version: "1.0.0",
								Order:   20,
							},
						},
					},
				},
			},
			want: &v1beta1.Helm{
				ConcurrencyLevel: 1,
				Repositories:     nil,
				Charts: []v1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Order:   120,
					},
					{
						Name:         "openebs",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
						Version:      "1.2.3-openebs",
						Values:       test_openebsValues,
						TargetNS:     "openebs",
						ForceUpgrade: ptr.To(false),
						Order:        101,
					},
					{
						Name:         "embedded-cluster-operator",
						ChartName:    "test-operator-location",
						Version:      "1.2.3-operator",
						Values:       test_operatorValues,
						TargetNS:     "embedded-cluster",
						ForceUpgrade: ptr.To(false),
						Order:        103,
					},
					{
						Name:         "admin-console",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
						Version:      "1.2.3-admin-console",
						Values:       test_onlineAdminConsoleValues,
						TargetNS:     "kotsadm",
						ForceUpgrade: ptr.To(false),
						Order:        105,
					},
				},
			},
		},
		{
			name:             "online non-ha velero",
			airgap:           false,
			highAvailability: false,
			disasterRecovery: true,
			args: args{
				in: v1beta1.Extensions{
					Helm: &v1beta1.Helm{
						ConcurrencyLevel: 2,
						Repositories:     nil,
						Charts: []v1beta1.Chart{
							{
								Name:    "test",
								Version: "1.0.0",
								Order:   20,
							},
						},
					},
				},
			},
			want: &v1beta1.Helm{
				ConcurrencyLevel: 1,
				Repositories:     nil,
				Charts: []v1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Order:   120,
					},
					{
						Name:         "openebs",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
						Version:      "1.2.3-openebs",
						Values:       test_openebsValues,
						TargetNS:     "openebs",
						ForceUpgrade: ptr.To(false),
						Order:        101,
					},
					{
						Name:         "embedded-cluster-operator",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
						Version:      "1.2.3-operator",
						Values:       test_operatorValues,
						TargetNS:     "embedded-cluster",
						ForceUpgrade: ptr.To(false),
						Order:        103,
					},
					{
						Name:         "velero",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/velero",
						Version:      "1.2.3-velero",
						Values:       test_veleroValues,
						TargetNS:     "velero",
						ForceUpgrade: ptr.To(false),
						Order:        103,
					},
					{
						Name:         "admin-console",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
						Version:      "1.2.3-admin-console",
						Values:       test_onlineAdminConsoleValues,
						TargetNS:     "kotsadm",
						ForceUpgrade: ptr.To(false),
						Order:        105,
					},
				},
			},
		},
		{
			name:             "airgap, non-ha, no-velero",
			airgap:           true,
			highAvailability: false,
			disasterRecovery: false,
			args: args{
				in: v1beta1.Extensions{},
			},
			want: &v1beta1.Helm{
				ConcurrencyLevel: 1,
				Repositories:     nil,
				Charts: []v1beta1.Chart{
					{
						Name:         "openebs",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
						Version:      "1.2.3-openebs",
						Values:       test_openebsValues,
						TargetNS:     "openebs",
						ForceUpgrade: ptr.To(false),
						Order:        101,
					},
					{
						Name:         "docker-registry",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/docker-registry",
						Version:      "1.2.3-registry",
						Values:       test_registryValues,
						TargetNS:     "registry",
						ForceUpgrade: ptr.To(false),
						Order:        103,
					},
					{
						Name:         "embedded-cluster-operator",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
						Version:      "1.2.3-operator",
						Values:       test_operatorValues,
						TargetNS:     "embedded-cluster",
						ForceUpgrade: ptr.To(false),
						Order:        103,
					},
					{
						Name:         "admin-console",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
						Version:      "1.2.3-admin-console",
						Values:       test_airgapAdminConsoleValues,
						TargetNS:     "kotsadm",
						ForceUpgrade: ptr.To(false),
						Order:        105,
					},
				},
			},
		},
		{
			name:             "ha airgap enabled, migration incomplete",
			airgap:           true,
			highAvailability: true,
			args: args{
				in: v1beta1.Extensions{},
				conditions: []metav1.Condition{
					{
						Type:   registry.RegistryMigrationStatusConditionType,
						Status: metav1.ConditionFalse,
						Reason: "MigrationInProgress",
					},
				},
			},
			want: &v1beta1.Helm{
				ConcurrencyLevel: 1,
				Repositories:     nil,
				Charts: []v1beta1.Chart{
					{
						Name:         "openebs",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
						Version:      "1.2.3-openebs",
						Values:       test_openebsValues,
						TargetNS:     "openebs",
						ForceUpgrade: ptr.To(false),
						Order:        101,
					},
					{
						Name:         "docker-registry",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/docker-registry",
						Version:      "1.2.3-registry",
						Values:       test_registryValues,
						TargetNS:     "registry",
						ForceUpgrade: ptr.To(false),
						Order:        103,
					},
					{
						Name:         "seaweedfs",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/seaweedfs",
						Version:      "1.2.3-seaweedfs",
						Values:       test_seaweedfsValues,
						TargetNS:     "seaweedfs",
						ForceUpgrade: ptr.To(false),
						Order:        102,
					},
					{
						Name:         "embedded-cluster-operator",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
						Version:      "1.2.3-operator",
						Values:       test_operatorValues,
						TargetNS:     "embedded-cluster",
						ForceUpgrade: ptr.To(false),
						Order:        103,
					},
					{
						Name:         "admin-console",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
						Version:      "1.2.3-admin-console",
						Values:       test_airgapHAAdminConsoleValues,
						TargetNS:     "kotsadm",
						ForceUpgrade: ptr.To(false),
						Order:        105,
					},
				},
			},
		},
		{
			name:             "ha airgap enabled, migration complete",
			airgap:           true,
			highAvailability: true,
			args: args{
				in: v1beta1.Extensions{},
				conditions: []metav1.Condition{
					{
						Type:   registry.RegistryMigrationStatusConditionType,
						Status: metav1.ConditionTrue,
						Reason: "MigrationComplete",
					},
				},
			},
			want: &v1beta1.Helm{
				ConcurrencyLevel: 1,
				Repositories:     nil,
				Charts: []v1beta1.Chart{
					{
						Name:         "openebs",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/openebs",
						Version:      "1.2.3-openebs",
						Values:       test_openebsValues,
						TargetNS:     "openebs",
						ForceUpgrade: ptr.To(false),
						Order:        101,
					},
					{
						Name:         "docker-registry",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/docker-registry",
						Version:      "1.2.3-registry",
						Values:       test_haRegistryValues,
						TargetNS:     "registry",
						ForceUpgrade: ptr.To(false),
						Order:        103,
					},
					{
						Name:         "seaweedfs",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/seaweedfs",
						Version:      "1.2.3-seaweedfs",
						Values:       test_seaweedfsValues,
						TargetNS:     "seaweedfs",
						ForceUpgrade: ptr.To(false),
						Order:        102,
					},
					{
						Name:         "embedded-cluster-operator",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator",
						Version:      "1.2.3-operator",
						Values:       test_operatorValues,
						TargetNS:     "embedded-cluster",
						ForceUpgrade: ptr.To(false),
						Order:        103,
					},
					{
						Name:         "admin-console",
						ChartName:    "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/admin-console",
						Version:      "1.2.3-admin-console",
						Values:       test_airgapHAAdminConsoleValues,
						TargetNS:     "kotsadm",
						ForceUpgrade: ptr.To(false),
						Order:        105,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installation := v1beta1.Installation{
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						Version:    "1.0.0",
						Extensions: tt.args.in,
					},
					AirGap:           tt.airgap,
					HighAvailability: tt.highAvailability,
					LicenseInfo: &v1beta1.LicenseInfo{
						IsDisasterRecoverySupported: tt.disasterRecovery,
					},
					ClusterID:  "e79f0701-67f3-4abf-a672-42a1f3ed231b",
					BinaryName: "test-binary-name",
				},
				Status: v1beta1.InstallationStatus{
					Conditions: tt.args.conditions,
				},
			}

			images := []string{
				"abc-repo/ec-utils:latest-amd64@sha256:92dec6e167ff57b35953da389c2f62c8ed9e529fe8dac3c3621269c3a66291f0",                                    // the utils image is needed
				"docker.io/replicated/another-image:latest-arm64@sha256:a9ab9db181f9898283a87be0f79d85cb8f3d22a790b71f52c8a9d339e225dedd",                   // this image is here to ensure we're actually searching
				"docker.io/replicated/embedded-cluster-operator-image:latest-amd64@sha256:eeed01216b5d2192afbd90e2e1f70419a8758551d8708f9d4b4f50f41d106ce8", // this operator image is needed
			}

			ol := "oci://proxy.replicated.com/anonymous/registry.replicated.com/library/embedded-cluster-operator"
			if tt.operatorLocation != "" {
				ol = tt.operatorLocation
			}

			req := require.New(t)
			got, err := generateHelmConfigs(context.TODO(), &installation, &tt.args.clusterConfig, images, ol)
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}

func Test_applyUserProvidedAddonOverrides(t *testing.T) {
	tests := []struct {
		name         string
		installation *v1beta1.Installation
		config       *v1beta1.Helm
		want         *v1beta1.Helm
	}{
		{
			name:         "no config",
			installation: &v1beta1.Installation{},
			config: &v1beta1.Helm{
				Charts: []v1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
			want: &v1beta1.Helm{
				Charts: []v1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
		},
		{
			name: "no override",
			installation: &v1beta1.Installation{
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						UnsupportedOverrides: v1beta1.UnsupportedOverrides{
							BuiltInExtensions: []v1beta1.BuiltInExtension{},
						},
					},
				},
			},
			config: &v1beta1.Helm{
				Charts: []v1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
			want: &v1beta1.Helm{
				Charts: []v1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
		},
		{
			name: "single addition",
			installation: &v1beta1.Installation{
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						UnsupportedOverrides: v1beta1.UnsupportedOverrides{
							BuiltInExtensions: []v1beta1.BuiltInExtension{
								{
									Name:   "test",
									Values: "foo: bar",
								},
							},
						},
					},
				},
			},
			config: &v1beta1.Helm{
				Charts: []v1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
			want: &v1beta1.Helm{
				Charts: []v1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz\nfoo: bar\n",
					},
				},
			},
		},
		{
			name: "single override",
			installation: &v1beta1.Installation{
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						UnsupportedOverrides: v1beta1.UnsupportedOverrides{
							BuiltInExtensions: []v1beta1.BuiltInExtension{
								{
									Name:   "test",
									Values: "abc: newvalue",
								},
							},
						},
					},
				},
			},
			config: &v1beta1.Helm{
				Charts: []v1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
				},
			},
			want: &v1beta1.Helm{
				Charts: []v1beta1.Chart{
					{
						Name:    "test",
						Version: "1.0.0",
						Values:  "abc: newvalue\n",
					},
				},
			},
		},
		{
			name: "multiple additions and overrides",
			installation: &v1beta1.Installation{
				Spec: v1beta1.InstallationSpec{
					Config: &v1beta1.ConfigSpec{
						UnsupportedOverrides: v1beta1.UnsupportedOverrides{
							BuiltInExtensions: []v1beta1.BuiltInExtension{
								{
									Name:   "chart0",
									Values: "added: added\noverridden: overridden",
								},
								{
									Name:   "chart1",
									Values: "foo: replacement",
								},
							},
						},
					},
				},
			},
			config: &v1beta1.Helm{
				ConcurrencyLevel: 999,
				Repositories: []v1beta1.Repository{
					{
						Name: "repo",
						URL:  "https://repo",
					},
				},
				Charts: []v1beta1.Chart{
					{
						Name:    "chart0",
						Version: "1.0.0",
						Values:  "abc: xyz",
					},
					{
						Name:    "chart1",
						Version: "1.0.0",
						Values:  "foo: bar",
					},
				},
			},
			want: &v1beta1.Helm{
				ConcurrencyLevel: 999,
				Repositories: []v1beta1.Repository{
					{
						Name: "repo",
						URL:  "https://repo",
					},
				},
				Charts: []v1beta1.Chart{
					{
						Name:    "chart0",
						Version: "1.0.0",
						Values:  "abc: xyz\nadded: added\noverridden: overridden\n",
					},
					{
						Name:    "chart1",
						Version: "1.0.0",
						Values:  "foo: replacement\n",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			got, err := applyUserProvidedAddonOverrides(tt.installation, tt.config)
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}
