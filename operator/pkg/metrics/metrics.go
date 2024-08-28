package metrics

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// NodeEventFromNode returns a NodeEvent event from a node.
func NodeEventFromNode(clusterID string, node corev1.Node) NodeEvent {
	return NodeEvent{
		ClusterID:   clusterID,
		Labels:      node.Labels,
		NodeName:    node.Name,
		Info:        node.Status.NodeInfo,
		Capacity:    node.Status.Capacity,
		Allocatable: node.Status.Allocatable,
		Role:        node.Labels["node.k0sproject.io/role"],
	}
}

// NodeEvent is the event generated when a new node appears in the cluster
// or an existing node is updated.
type NodeEvent struct {
	ClusterID   string                `json:"clusterID"`
	Labels      map[string]string     `json:"labels"`
	NodeName    string                `json:"nodeName"`
	Info        corev1.NodeSystemInfo `json:"info"`
	Capacity    corev1.ResourceList   `json:"capacity"`
	Allocatable corev1.ResourceList   `json:"allocatable"`
	Role        string                `json:"role"`
}

// UpgradeStartedEvent is send back home when the upgrade starts.
type UpgradeStartedEvent struct {
	ClusterID string `json:"clusterID"`
	Version   string `json:"version"`
}

// UpgradeFailedEvent is send back home when the upgrade fails.
type UpgradeFailedEvent struct {
	ClusterID string `json:"clusterID"`
	Reason    string `json:"reason"`
}

// UpgradeSucceededEvent event is send back home when the upgrade succeeds.
type UpgradeSucceededEvent struct {
	ClusterID string `json:"clusterID"`
}

// Hash returns the hash of the node.
func (n NodeEvent) Hash() (string, error) {
	hasher := sha256.New()
	if err := json.NewEncoder(hasher).Encode(n); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// NodeRemovedEvent is the event generated when a node is removed from the cluster.
type NodeRemovedEvent struct {
	ClusterID string `json:"clusterID"`
	NodeName  string `json:"nodeName"`
}

// sendEvent sends the received event to the metrics server through a post request.
func sendEvent(ctx context.Context, evname, baseURL string, ev interface{}) error {
	url := fmt.Sprintf("%s/embedded_cluster_metrics/%s", baseURL, evname)
	body := map[string]interface{}{"event": ev}
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(body); err != nil {
		return err
	}
	ictx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ictx, http.MethodPost, url, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send event: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send event: %s", resp.Status)
	}
	return nil
}

// NotifyNodeUpdated notifies the metrics server that a node has been updated.
func NotifyNodeUpdated(ctx context.Context, baseURL string, ev NodeEvent) error {
	return sendEvent(ctx, "NodeUpdated", baseURL, ev)
}

// NotifyNodeAdded notifies the metrics server that a node has been added.
func NotifyNodeAdded(ctx context.Context, baseURL string, ev NodeEvent) error {
	return sendEvent(ctx, "NodeAdded", baseURL, ev)
}

// NotifyNodeRemoved notifies the metrics server that a node has been removed.
func NotifyNodeRemoved(ctx context.Context, baseURL string, ev NodeRemovedEvent) error {
	return sendEvent(ctx, "NodeRemoved", baseURL, ev)
}

// NotifyUpgradeStarted notifies the metrics server that an upgrade has started.
func NotifyUpgradeStarted(ctx context.Context, baseURL string, ev UpgradeStartedEvent) error {
	return sendEvent(ctx, "UpgradeStarted", baseURL, ev)
}

// NotifyUpgradeFailed notifies the metrics server that an upgrade has failed.
func NotifyUpgradeFailed(ctx context.Context, baseURL string, ev UpgradeFailedEvent) error {
	return sendEvent(ctx, "UpgradeFailed", baseURL, ev)
}

// NotifyUpgradeSucceeded notifies the metrics server that an upgrade has succeeded.
func NotifyUpgradeSucceeded(ctx context.Context, baseURL string, ev UpgradeSucceededEvent) error {
	return sendEvent(ctx, "UpgradeSucceeded", baseURL, ev)
}
