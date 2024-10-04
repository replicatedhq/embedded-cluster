package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestInstallationReconciler_constructCreateCMCommand(t *testing.T) {
	job := constructHostPreflightResultsJob("my-node", "install-name")
	require.Len(t, job.Spec.Template.Spec.Containers, 1)
	require.Len(t, job.Spec.Template.Spec.Containers[0].Command, 4)
	kctlCmd := job.Spec.Template.Spec.Containers[0].Command[3]
	expected := "if [ -f /var/lib/embedded-cluster/support/host-preflight-results.json ]; then /var/lib/embedded-cluster/bin/kubectl create configmap ${HSPF_CM_NAME} --from-file=results.json=/var/lib/embedded-cluster/support/host-preflight-results.json -n embedded-cluster --dry-run=client -oyaml | /var/lib/embedded-cluster/bin/kubectl label -f - embedded-cluster/host-preflight-result=${EC_NODE_NAME} --local -o yaml | /var/lib/embedded-cluster/bin/kubectl apply -f - && /var/lib/embedded-cluster/bin/kubectl annotate configmap ${HSPF_CM_NAME} \"update-timestamp=$(date +'%Y-%m-%dT%H:%M:%SZ')\" --overwrite; else echo '/var/lib/embedded-cluster/support/host-preflight-results.json does not exist'; fi"
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
