# Embedded Cluster Custom Resources
Controlling resources utilized inside embedded cluster

## ClusterConfig
- **Owner**: k0s
- **Namespace**: kube-system

The `ClusterConfig` object contains the ingested k0s config from `/etc/k0s/k0s.yaml`

This ingestion happens at k0s daemon startup on controller nodes.

It can be dynamically updated, and the k0s config reconciliation process will apply any changes to the cluster except from `spec.api` and `spec.storage`.

The [embedded cluster operator](https://github.com/replicatedhq/embedded-cluster-operator/) reconciles helm chart updates against the `ClusterConfig` object to initiate helm chart upgrades via the k0s helm reconciler.

## Chart
- **Owner**: k0s
- **Namespace**: kube-system

The `Chart` object contains the spec, values and tracking information for helm charts installed by the [k0s helm reconciler](https://docs.k0sproject.io/head/helm-charts/); they can be created, deleted and updated. Deleting a `Chart` object will uninstall the related helm chart from the cluster, however if the helm chart configuration is still present in the `ClusterConfig` the k0s reconciliation process will recreate/reinstall it.

`Chart` Objects are currently only managed by the k0s helm reconciler, and it’s best to leave it that way for now as the API / schema for these resources is not documented.

`Chart` Objects are monitored by the Embedded Cluster Operator in order to track and surface helm installation processes and errors.

## Plan
- **Owner**: k0s
- **Cluster scoped object**

`Plan` objects are used to configure the k0s autopilot operator, the autopilot operator controls cluster version upgrades via distributing and installing new k0s binaries and airgap bundles.

The `Plan` resource is created by the Embedded Cluster Operator using details from the `Installation` object.

## Installation
- **Owner**: Replicated
- **Cluster scoped object**

The `Installation` object is used by the Embedded Cluster Operator to both initiate and track cluster and helm chart upgrades. they are created by [KOTS](https://github.com/replicatedhq/kots), and are marked as `Obsolete` when superseded by a newer `Installation` object.

The `Installation` object can contain errors surfaced from the `Plan` and `Chart` resources.

Possible Installation statuses can be found here: https://github.com/replicatedhq/embedded-cluster-operator/blob/e4fbb42919ad3b58cdc563dca77471cf76099393/api/v1beta1/installation_types.go#L24
