package controllers

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInstallationReconciler_constructCreateCMCommand(t *testing.T) {
	in := &v1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "install-name",
		},
		Spec: v1beta1.InstallationSpec{
			RuntimeConfig: &v1beta1.RuntimeConfigSpec{
				DataDir: "/var/lib/embedded-cluster",
			},
		},
	}
	job := constructHostPreflightResultsJob(in, "my-node")
	require.Len(t, job.Spec.Template.Spec.Volumes, 1)
	require.Equal(t, "/var/lib/embedded-cluster", job.Spec.Template.Spec.Volumes[0].VolumeSource.HostPath.Path)
	require.Len(t, job.Spec.Template.Spec.Containers, 1)
	require.Len(t, job.Spec.Template.Spec.Containers[0].Command, 4)
	kctlCmd := job.Spec.Template.Spec.Containers[0].Command[3]
	expected := "if [ -f /embedded-cluster/support/host-preflight-results.json ]; then /embedded-cluster/bin/kubectl create configmap ${HSPF_CM_NAME} --from-file=results.json=/embedded-cluster/support/host-preflight-results.json -n embedded-cluster --dry-run=client -oyaml | /embedded-cluster/bin/kubectl label -f - embedded-cluster/host-preflight-result=${EC_NODE_NAME} --local -o yaml | /embedded-cluster/bin/kubectl apply -f - && /embedded-cluster/bin/kubectl annotate configmap ${HSPF_CM_NAME} \"update-timestamp=$(date +'%Y-%m-%dT%H:%M:%SZ')\" --overwrite; else echo '/embedded-cluster/support/host-preflight-results.json does not exist'; fi"
	assert.Equal(t, expected, kctlCmd)
	require.Len(t, job.Spec.Template.Spec.Containers[0].Env, 2)
	assert.Equal(t, v1.EnvVar{
		Name:  "EC_NODE_NAME",
		Value: "my-node",
	}, job.Spec.Template.Spec.Containers[0].Env[0])
	assert.Equal(t, v1.EnvVar{
		Name:  "HSPF_CM_NAME",
		Value: "my-node-host-preflight-results",
	}, job.Spec.Template.Spec.Containers[0].Env[1])
}
