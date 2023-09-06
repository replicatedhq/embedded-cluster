package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/hashicorp/terraform-exec/tfexec"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
	"github.com/stretchr/testify/assert"
)

func writeTo(w io.Writer) func(string, ...any) (int, error) {
	return func(format string, a ...any) (int, error) {
		return fmt.Fprintf(w, format, a...)
	}
}

var validTerraformOutput = []byte(`
[
	{
		"address": "192.168.0.1",
		"role":  "controller",
		"port": 22,
		"user": "user1",
		"keyPath": "/home/user/.ssh/key1.pem"
	},
	{
		"address": "192.168.0.2",
		"role":  "controller+worker",
		"port": 23,
		"user": "user2",
		"keyPath": "/home/user/.ssh/key2.pem"
	},
	{
		"address": "192.168.0.3",
		"role":  "worker",
		"port": 24,
		"user": "user3",
		"keyPath": "/home/user/.ssh/key3.pem"
	}
]
`)

func TestApply(t *testing.T) {
	infra := New()
	infra.apply = func(context.Context, string, pb.MessageWriter) (map[string]tfexec.OutputMeta, error) {
		return nil, fmt.Errorf("test error")
	}
	_, err := infra.Apply(context.Background(), "", false)
	assert.ErrorContains(t, err, "test error", "error should contain 'test error'")

	buf := bytes.NewBuffer(nil)
	infra.printf = writeTo(buf)
	infra.apply = func(context.Context, string, pb.MessageWriter) (map[string]tfexec.OutputMeta, error) {
		return map[string]tfexec.OutputMeta{"nodes": {Value: []byte(`[]`)}}, nil
	}
	_, err = infra.Apply(context.Background(), "", false)
	assert.ErrorContains(t, err, "no nodes found in terraform output")

	buf = bytes.NewBuffer(nil)
	infra.printf = writeTo(buf)
	infra.apply = func(context.Context, string, pb.MessageWriter) (map[string]tfexec.OutputMeta, error) {
		return map[string]tfexec.OutputMeta{"nodes": {Value: []byte(`{this is not a valid/json{`)}}, nil
	}
	_, err = infra.Apply(context.Background(), "", false)
	assert.ErrorContains(t, err, "unable to unmarshal terraform output")

	buf = bytes.NewBuffer(nil)
	infra.printf = writeTo(buf)
	infra.apply = func(context.Context, string, pb.MessageWriter) (map[string]tfexec.OutputMeta, error) {
		return map[string]tfexec.OutputMeta{"nodes": {Value: validTerraformOutput}}, nil
	}
	nodes, err := infra.Apply(context.Background(), "", false)
	assert.NoError(t, err)
	data, err := json.Marshal(nodes)
	assert.NoError(t, err)
	assert.JSONEq(t, string(validTerraformOutput), string(data))
	for _, expected := range []string{
		"192.168.0.1",
		"192.168.0.2",
		"192.168.0.3",
		"22",
		"23",
		"24",
		"user1",
		"user2",
		"user3",
		"/home/user/.ssh/key1.pem",
		"/home/user/.ssh/key2.pem",
		"/home/user/.ssh/key3.pem",
		"controller+worker",
		"worker",
	} {
		assert.Contains(t, buf.String(), expected)
	}
}
