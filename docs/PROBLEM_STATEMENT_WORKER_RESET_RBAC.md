# Problem Statement: Worker Node Self-Delete During Reset

## The Problem

Kubernetes' NodeRestriction admission plugin blocks kubelet identities from deleting nodes — even their own node object. Kubelets can only read and update their node object. This means that when a **worker node** runs `reset` and attempts to delete its own Node from the cluster, it receives a `Forbidden` error.

## Impact

Before this fix, workers resetting from the cluster would:
1. Successfully drain themselves.
2. Fail to delete their Node object from the API server.
3. Leave a stale Node object in etcd.
4. Require a manual `kubectl delete node <name>` from a surviving controller node to clean up.

This required a user-facing prompt in the reset CLI:

> "Unable to delete this worker node from the API server due to insufficient permissions."  
> "To complete the reset, remove this node from the cluster by running 'kubectl delete node <name>' from a surviving controller node."

The user had to confirm they wanted to continue with a local-only reset, leaving the stale Node behind.

## Root Cause

The kubelet certificate used by workers for Kubernetes API authentication is bound by the NodeRestriction admission plugin. This is a **security feature** of Kubernetes — kubelets are not allowed to delete Nodes, even their own. Only identities with explicit RBAC permissions can delete Node objects.

## Solution Overview

We implemented **per-node RBAC** so each worker gets a dedicated ServiceAccount scoped to delete only its own Node:

### 1. Controller-Side: RBAC Provisioning

During the operator's reconcile loop (`InstallationReconciler.Reconcile`), for every **current worker node** in the cluster:

- Create a `ServiceAccount`: `node-self-delete-<hostname>`
- Create a `ClusterRole` scoped to **only that node**:
  ```yaml
  rules:
    - apiGroups: [""]
      resources: ["nodes"]
      resourceNames: ["<hostname>"]
      verbs: ["delete"]
  ```
- Create a `ClusterRoleBinding` linking the SA to the role.
- Create a `Secret` (type `kubernetes.io/service-account-token`) so Kubernetes auto-populates a token.

When a node is removed from the cluster, these RBAC resources are cleaned up automatically.

### 2. Token Delivery to the Worker

A one-shot Kubernetes Job is created (pinned to the target node via `NodeName`) that:
- Waits for the Secret's `.data.token` to be populated.
- Writes the decoded token to `/var/lib/embedded-cluster/node-delete-token` with `600` permissions.

This ensures the worker has its own self-delete token available locally.

### 3. Reset Flow: Automatic Token Fallback

During `reset`, when a worker attempts to delete its Node:
1. First, it tries with the kubelet identity (this will fail with `Forbidden`).
2. On `Forbidden`, it falls back to reading `/var/lib/embedded-cluster/node-delete-token`.
3. It builds a temporary kubeconfig using the SA token + the cluster CA/server from the existing kubelet config.
4. It retries the Node deletion with the ServiceAccount identity, which **succeeds** because the ClusterRole is explicitly scoped to allow deletion of that specific Node.

### 4. Retroactive Coverage

The reconcile loop runs for **all current worker nodes**, not just newly added ones. This means:
- Existing clusters get per-node RBAC automatically (no migration needed).
- Nodes that joined before this feature was deployed will get their RBAC and token delivered on the next reconcile cycle.

## Result

The manual `kubectl delete node` step and the user-facing prompt are **eliminated**. Workers can fully self-delete during reset without leaving stale Node objects or requiring intervention from a controller node.
