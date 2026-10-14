package seaweedfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	_masterStatefulSetName = "seaweedfs-master"
	_containerName         = "seaweedfs"
	_masterPort            = 9333
)

// ClusterCheck runs "cluster.check" in the weed shell of the first master pod and returns an
// error if any filer cannot reach another filer. A filer pings another filer only when it follows
// that filer's metadata, so this catches a filer that started without its peers (for example
// when the master's DNS name did not resolve yet). Such a filer never learns about filers that
// joined before it and serves S3 requests from incomplete metadata, failing multipart uploads
// started on another filer with NoSuchUpload.
func ClusterCheck(ctx context.Context, kcli client.Client) error {
	var sts appsv1.StatefulSet
	if err := kcli.Get(ctx, client.ObjectKey{Namespace: _namespace, Name: _masterStatefulSetName}, &sts); err != nil {
		return fmt.Errorf("get seaweedfs master statefulset: %w", err)
	}
	replicas := int32(1)
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}

	// the shell needs the masters' cluster addresses, as cluster.check pings the masters by the
	// addresses it was given and the other components only accept pings to masters they know
	masters := make([]string, 0, replicas)
	for i := range replicas {
		masters = append(masters, fmt.Sprintf("%s-%d.%s.%s:%d", _masterStatefulSetName, i, _masterStatefulSetName, _namespace, _masterPort))
	}

	command := []string{"weed", "shell", "-master=" + strings.Join(masters, ",")}
	stdout, stderr, err := kubeutils.ExecInPod(
		ctx, _namespace, _masterStatefulSetName+"-0", _containerName, command, strings.NewReader("cluster.check\n"),
	)
	if err != nil {
		return fmt.Errorf("run cluster.check: %w: %s", err, strings.TrimSpace(stderr))
	}
	return parseClusterCheckOutput(stdout)
}

// parseClusterCheckOutput returns an error if the cluster.check output has no filer-to-filer
// checks or any of them failed. weed shell exits zero when a ping fails and prints the error in
// place of the round trip time, so the output is the only signal. Checks between other components
// are ignored: a filer rejects pings to volume servers it has no volumes on, even when healthy.
func parseClusterCheckOutput(output string) error {
	var checked int
	var failures []string
	for line := range strings.Lines(output) {
		line = strings.TrimSpace(line)
		if !isFilerToFilerCheck(line) {
			continue
		}
		checked++
		if _, result, _ := strings.Cut(line, " ... "); !strings.HasPrefix(result, "ok ") {
			failures = append(failures, line)
		}
	}
	if checked == 0 {
		return fmt.Errorf("no filer to filer checks in cluster.check output: %s", strings.TrimSpace(output))
	}
	if len(failures) > 0 {
		return fmt.Errorf("filer to filer checks failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

// isFilerToFilerCheck reports whether a cluster.check output line is a ping from one filer to
// another, e.g. "checking filer 10.0.0.1:8888.18888 to 10.0.0.2:8888.18888 ... ok ...".
func isFilerToFilerCheck(line string) bool {
	rest, ok := strings.CutPrefix(line, "checking filer ")
	if !ok {
		return false
	}
	_, target, ok := strings.Cut(rest, " to ")
	if !ok {
		return false
	}
	return !strings.HasPrefix(target, "master ") && !strings.HasPrefix(target, "volume server ")
}
