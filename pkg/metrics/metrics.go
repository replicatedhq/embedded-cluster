package metrics

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
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

// sendEvent sends the received buf to the provided url through a post request. Buffer
// is expected to be a json encoded object.
func sendEvent(ctx context.Context, url string, body io.Reader) error {
	ictx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ictx, http.MethodPost, url, body)
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
	url := fmt.Sprintf("%s/helmbin_metrics/NodeUpdated", baseURL)
	body := map[string]interface{}{"event": ev}
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(body); err != nil {
		return err
	}
	return sendEvent(ctx, url, buf)
}

// NotifyNodeAdded notifies the metrics server that a node has been added.
func NotifyNodeAdded(ctx context.Context, baseURL string, ev NodeEvent) error {
	url := fmt.Sprintf("%s/helmbin_metrics/NodeAdded", baseURL)
	body := map[string]interface{}{"event": ev}
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(body); err != nil {
		return err
	}
	return sendEvent(ctx, url, buf)
}

// NotifyNodeRemoved notifies the metrics server that a node has been removed.
func NotifyNodeRemoved(ctx context.Context, baseURL string, ev NodeRemovedEvent) error {
	url := fmt.Sprintf("%s/helmbin_metrics/NodeRemoved", baseURL)
	body := map[string]interface{}{"event": ev}
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(body); err != nil {
		return err
	}
	return sendEvent(ctx, url, buf)
}
