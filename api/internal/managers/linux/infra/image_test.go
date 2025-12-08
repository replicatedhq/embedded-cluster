package infra

import (
	"fmt"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/require"
)

func Test_DestECImage(t *testing.T) {
	registryOps := &types.RegistrySettings{
		LocalRegistryHost:      "localhost:5000",
		LocalRegistryNamespace: "somebigbank",
	}

	type args struct {
		registry *types.RegistrySettings
		srcImage string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "ECR style image",
			args: args{
				registry: registryOps,
				srcImage: "411111111111.dkr.ecr.us-west-1.amazonaws.com/myrepo:v0.0.1",
			},
			want: fmt.Sprintf("%s/%s/embedded-cluster/myrepo:v0.0.1", registryOps.LocalRegistryHost, registryOps.LocalRegistryNamespace),
		},
		{
			name: "Quay image with tag",
			args: args{
				registry: registryOps,
				srcImage: "quay.io/someorg/debian:0.1",
			},
			want: fmt.Sprintf("%s/%s/embedded-cluster/debian:0.1", registryOps.LocalRegistryHost, registryOps.LocalRegistryNamespace),
		},
		{
			name: "Quay image with digest",
			args: args{
				registry: registryOps,
				srcImage: "quay.io/someorg/debian@sha256:17c5f462c92fc39303e6363c65e074559f8d6a1354150027ed5053557e3298c5",
			},
			want: fmt.Sprintf("%s/%s/embedded-cluster/debian@sha256:17c5f462c92fc39303e6363c65e074559f8d6a1354150027ed5053557e3298c5", registryOps.LocalRegistryHost, registryOps.LocalRegistryNamespace),
		},
		{
			name: "Image with tag and digest",
			args: args{
				registry: registryOps,
				srcImage: "quay.io/someorg/debian:0.1@sha256:17c5f462c92fc39303e6363c65e074559f8d6a1354150027ed5053557e3298c5",
			},
			want: fmt.Sprintf("%s/%s/embedded-cluster/debian@sha256:17c5f462c92fc39303e6363c65e074559f8d6a1354150027ed5053557e3298c5", registryOps.LocalRegistryHost, registryOps.LocalRegistryNamespace),
		},
		{
			name: "No Namespace",
			args: args{
				registry: &types.RegistrySettings{
					LocalRegistryHost: "localhost:5000",
				},
				srcImage: "quay.io/someorg/debian:0.1",
			},
			want: fmt.Sprintf("%s/embedded-cluster/debian:0.1", registryOps.LocalRegistryHost),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			got, err := destECImage(tt.args.registry, tt.args.srcImage)
			req.NoError(err)

			if got != tt.want {
				t.Errorf("DestECImage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeTag(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Valid repos
		{"1.0.1", "1.0.1"},
		{"my-App123", "my-App123"},
		{"123-456", "123-456"},
		{"my-App123.-", "my-App123.-"},
		{"my-App123-.", "my-App123-."},

		// Invalid repos
		{".invalid", "invalid"},
		{"-invalid", "invalid"},
		{"not valid!", "notvalid"},

		// Tags longer than 128 characters
		{"0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789", "01234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			sanitized := sanitizeTag(test.input)
			if sanitized != test.expected {
				t.Errorf("got: %s, expected: %s", sanitized, test.expected)
			}
		})
	}
}

func TestSanitizeRepo(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Valid repos
		{"nginx", "nginx"},
		{"my-app-123", "my-app-123"},
		{"my_app_123", "my_app_123"},
		{"charts.tar.gz", "charts.tar.gz"},

		// Invalid repos
		{"My-App-123", "my-app-123"},
		{"-invalid", "invalid"},
		{"_invalid", "invalid"},
		{".invalid", "invalid"},
		{"not valid!", "notvalid"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			sanitized := sanitizeRepo(test.input)
			if sanitized != test.expected {
				t.Errorf("got: %s, expected: %s", sanitized, test.expected)
			}
		})
	}
}

func TestECArtifactOCIPath(t *testing.T) {
	type args struct {
		filename string
		opts     ECArtifactOCIPathOptions
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "happy path for binary",
			args: args{
				filename: "embedded-cluster/embedded-cluster-amd64",
				opts: ECArtifactOCIPathOptions{
					RegistryHost:      "registry.example.com",
					RegistryNamespace: "my-app",
					ChannelID:         "test-channel-id",
					UpdateCursor:      "1",
					VersionLabel:      "1.0.0",
				},
			},
			want: "registry.example.com/my-app/embedded-cluster/embedded-cluster-amd64:test-channel-id-1-1.0.0",
		},
		{
			name: "happy path for charts bundle",
			args: args{
				filename: "embedded-cluster/charts.tar.gz",
				opts: ECArtifactOCIPathOptions{
					RegistryHost:      "registry.example.com",
					RegistryNamespace: "my-app",
					ChannelID:         "test-channel-id",
					UpdateCursor:      "1",
					VersionLabel:      "1.0.0",
				},
			},
			want: "registry.example.com/my-app/embedded-cluster/charts.tar.gz:test-channel-id-1-1.0.0",
		},
		{
			name: "happy path for image bundle",
			args: args{
				filename: "embedded-cluster/images-amd64.tar",
				opts: ECArtifactOCIPathOptions{
					RegistryHost:      "registry.example.com",
					RegistryNamespace: "my-app",
					ChannelID:         "test-channel-id",
					UpdateCursor:      "1",
					VersionLabel:      "1.0.0",
				},
			},
			want: "registry.example.com/my-app/embedded-cluster/images-amd64.tar:test-channel-id-1-1.0.0",
		},
		{
			name: "happy path for version metadata",
			args: args{
				filename: "embedded-cluster/version-metadata.json",
				opts: ECArtifactOCIPathOptions{
					RegistryHost:      "registry.example.com",
					RegistryNamespace: "my-app",
					ChannelID:         "test-channel-id",
					UpdateCursor:      "1",
					VersionLabel:      "1.0.0",
				},
			},
			want: "registry.example.com/my-app/embedded-cluster/version-metadata.json:test-channel-id-1-1.0.0",
		},
		{
			name: "file with name that needs to be sanitized",
			args: args{
				filename: "A file with spaces.tar.gz",
				opts: ECArtifactOCIPathOptions{
					RegistryHost:      "registry.example.com",
					RegistryNamespace: "my-app",
					ChannelID:         "test-channel-id",
					UpdateCursor:      "1",
					VersionLabel:      "1.0.0",
				},
			},
			want: "registry.example.com/my-app/embedded-cluster/afilewithspaces.tar.gz:test-channel-id-1-1.0.0",
		},
		{
			name: "version label name that needs to be sanitized",
			args: args{
				filename: "test.txt",
				opts: ECArtifactOCIPathOptions{
					RegistryHost:      "registry.example.com",
					RegistryNamespace: "my-app",
					ChannelID:         "test-channel-id",
					UpdateCursor:      "1",
					VersionLabel:      "A version with spaces",
				},
			},
			want: "registry.example.com/my-app/embedded-cluster/test.txt:test-channel-id-1-Aversionwithspaces",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newECOCIArtifactPath(tt.args.filename, tt.args.opts); got.String() != tt.want {
				t.Errorf("ECArtifactOCIPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
