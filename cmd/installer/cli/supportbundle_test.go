package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBuildRedactorURIs(t *testing.T) {
	ctx := context.Background()
	appSlug := "my-app"

	cases := []struct {
		name          string
		secrets       []string
		expectedURIs  []string
		expectedWarns int
	}{
		{
			name: "all redactor secrets exist",
			secrets: []string{
				"kotsadm-redact-spec",
				"kotsadm-redact-default-spec",
				"kotsadm-my-app-redact-spec",
			},
			expectedURIs: []string{
				"secret/kotsadm/kotsadm-redact-spec/redact-spec",
				"secret/kotsadm/kotsadm-my-app-redact-spec/redact-spec",
				"secret/kotsadm/kotsadm-redact-default-spec/default-redactor",
			},
		},
		{
			name: "only admin console and default redactors exist",
			secrets: []string{
				"kotsadm-redact-spec",
				"kotsadm-redact-default-spec",
			},
			expectedURIs: []string{
				"secret/kotsadm/kotsadm-redact-spec/redact-spec",
				"secret/kotsadm/kotsadm-redact-default-spec/default-redactor",
			},
		},
		{
			name: "only app-specific redactor exists",
			secrets: []string{
				"kotsadm-my-app-redact-spec",
			},
			expectedURIs: []string{
				"secret/kotsadm/kotsadm-my-app-redact-spec/redact-spec",
			},
		},
		{
			name:         "no redactor secrets exist",
			secrets:      []string{},
			expectedURIs: []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			objects := make([]runtime.Object, 0, len(tc.secrets))
			for _, name := range tc.secrets {
				objects = append(objects, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: kotsadmNamespace,
					},
				})
			}
			clientset := fake.NewSimpleClientset(objects...)

			got := buildRedactorURIs(ctx, appSlug, clientset)
			assert.Equal(t, tc.expectedURIs, got)
		})
	}
}
