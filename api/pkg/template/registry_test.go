package template

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestKubernetesRegistryDetector_DetectRegistrySettings(t *testing.T) {
	tests := []struct {
		name         string
		objects      []client.Object
		license      *kotsv1beta1.License
		wantSettings *RegistrySettings
		wantError    bool
	}{
		{
			name:    "no registry deployment should return empty settings",
			objects: []client.Object{},
			license: &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{AppSlug: "my-app"},
			},
			wantSettings: &RegistrySettings{
				HasLocalRegistry:    false,
				Host:                "",
				Namespace:           "",
				Address:             "",
				ImagePullSecretName: "",
			},
			wantError: false,
		},
		{
			name: "deployment exists but service missing should return partial settings",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry",
						Namespace: constants.RegistryNamespace,
					},
				},
			},
			license: &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{AppSlug: "my-app"},
			},
			wantSettings: &RegistrySettings{
				HasLocalRegistry:    true,
				Host:                "",
				Namespace:           "",
				Address:             "",
				ImagePullSecretName: "",
			},
			wantError: false,
		},
		{
			name: "deployment and service exist should return full settings",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry",
						Namespace: constants.RegistryNamespace,
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry",
						Namespace: constants.RegistryNamespace,
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "10.96.0.10",
					},
				},
			},
			license: &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{AppSlug: "my-app"},
			},
			wantSettings: &RegistrySettings{
				HasLocalRegistry:    true,
				Host:                "10.96.0.10:5000",
				Namespace:           "my-app",
				Address:             "10.96.0.10:5000/my-app",
				ImagePullSecretName: "my-app-registry",
			},
			wantError: false,
		},
		{
			name: "deployment and service exist with no license should return settings without namespace",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry",
						Namespace: constants.RegistryNamespace,
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry",
						Namespace: constants.RegistryNamespace,
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "10.96.0.10",
					},
				},
			},
			license: nil,
			wantSettings: &RegistrySettings{
				HasLocalRegistry:    true,
				Host:                "10.96.0.10:5000",
				Namespace:           "",
				Address:             "10.96.0.10:5000",
				ImagePullSecretName: "",
			},
			wantError: false,
		},
		{
			name: "service with empty cluster IP should return partial settings",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry",
						Namespace: constants.RegistryNamespace,
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry",
						Namespace: constants.RegistryNamespace,
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "", // Empty ClusterIP
					},
				},
			},
			license: &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{AppSlug: "my-app"},
			},
			wantSettings: &RegistrySettings{
				HasLocalRegistry:    true,
				Host:                "",
				Namespace:           "",
				Address:             "",
				ImagePullSecretName: "",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with objects
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(tt.objects...).
				Build()

			logger := logrus.New()
			logger.SetLevel(logrus.DebugLevel)

			detector := NewKubernetesRegistryDetector(fakeClient, logger)
			ctx := context.Background()

			settings, err := detector.DetectRegistrySettings(ctx, tt.license)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantSettings, settings)
			}
		})
	}
}
