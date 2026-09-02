package cli

import (
	"context"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

type testLogHook struct {
	mu       sync.Mutex
	warnings []string
	disabled bool
}

func (h *testLogHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.WarnLevel}
}

func (h *testLogHook) Fire(entry *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.disabled {
		h.warnings = append(h.warnings, entry.Message)
	}
	return nil
}

func (h *testLogHook) Disable() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.disabled = true
}

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
			expectedWarns: 0,
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
			expectedWarns: 1,
		},
		{
			name: "only app-specific redactor exists",
			secrets: []string{
				"kotsadm-my-app-redact-spec",
			},
			expectedURIs: []string{
				"secret/kotsadm/kotsadm-my-app-redact-spec/redact-spec",
			},
			expectedWarns: 0,
		},
		{
			name:          "no redactor secrets exist",
			secrets:       []string{},
			expectedURIs:  []string{},
			expectedWarns: 1,
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

			hook := &testLogHook{}
			logrus.AddHook(hook)
			t.Cleanup(hook.Disable)

			got := buildRedactorURIs(ctx, appSlug, clientset)
			assert.Equal(t, tc.expectedURIs, got)
			assert.Len(t, hook.warnings, tc.expectedWarns)
		})
	}
}
