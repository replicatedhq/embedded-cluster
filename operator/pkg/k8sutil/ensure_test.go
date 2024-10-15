package k8sutil

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestEnsureObject(t *testing.T) {
	type args struct {
		obj       client.Object
		applyOpts []func(*EnsureObjectOptions)
	}
	tests := []struct {
		name            string
		initRuntimeObjs []client.Object
		args            args
		wantErr         bool
		assertObj       func(t *testing.T, obj client.Object)
	}{
		{
			name: "create object",
			initRuntimeObjs: []client.Object{
				&corev1.Namespace{
					TypeMeta: metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"},
					ObjectMeta: metav1.ObjectMeta{
						Name: "embedded-cluster",
					},
				},
			},
			args: args{
				obj: &corev1.Service{
					TypeMeta: metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "file-server",
						Namespace: "embedded-cluster",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name: "http",
								Port: 80,
							},
						},
					},
				},
			},
			wantErr: false,
			assertObj: func(t *testing.T, obj client.Object) {
				assert.IsType(t, &corev1.Service{}, obj)
				service := obj.(*corev1.Service)
				assert.Equal(t, "file-server", service.Name)
				assert.Equal(t, int32(80), service.Spec.Ports[0].Port)
			},
		},
		{
			name: "already exists",
			initRuntimeObjs: []client.Object{
				&corev1.Namespace{
					TypeMeta: metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"},
					ObjectMeta: metav1.ObjectMeta{
						Name: "embedded-cluster",
					},
				},
				&corev1.Service{
					TypeMeta: metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "file-server",
						Namespace: "embedded-cluster",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name: "http",
								Port: 80,
							},
						},
					},
				},
			},
			args: args{
				obj: &corev1.Service{
					TypeMeta: metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "file-server",
						Namespace: "embedded-cluster",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name: "http",
								Port: 8080,
							},
						},
					},
				},
			},
			wantErr: false,
			assertObj: func(t *testing.T, obj client.Object) {
				assert.IsType(t, &corev1.Service{}, obj)
				service := obj.(*corev1.Service)
				assert.Equal(t, "file-server", service.Name)
				assert.Equal(t, int32(80), service.Spec.Ports[0].Port)
			},
		},
		{
			name: "overwrite object",
			initRuntimeObjs: []client.Object{
				&corev1.Namespace{
					TypeMeta: metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"},
					ObjectMeta: metav1.ObjectMeta{
						Name: "embedded-cluster",
					},
				},
				&corev1.Service{
					TypeMeta: metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "file-server",
						Namespace: "embedded-cluster",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name: "http",
								Port: 80,
							},
						},
					},
				},
			},
			args: args{
				obj: &corev1.Service{
					TypeMeta: metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "file-server",
						Namespace: "embedded-cluster",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name: "http",
								Port: 8080,
							},
						},
					},
				},
				applyOpts: []func(*EnsureObjectOptions){
					func(opts *EnsureObjectOptions) {
						opts.ShouldDelete = func(obj client.Object) bool { return true }
					},
				},
			},
			wantErr: false,
			assertObj: func(t *testing.T, obj client.Object) {
				assert.IsType(t, &corev1.Service{}, obj)
				service := obj.(*corev1.Service)
				assert.Equal(t, "file-server", service.Name)
				assert.Equal(t, int32(8080), service.Spec.Ports[0].Port)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := testr.NewWithOptions(t, testr.Options{Verbosity: 10})
			ctx := logr.NewContext(context.Background(), log)

			testEnv := &envtest.Environment{}
			cfg, err := testEnv.Start()
			require.NoError(t, err)
			t.Cleanup(func() { _ = testEnv.Stop() })

			cli, err := client.New(cfg, client.Options{Scheme: Scheme()})
			require.NoError(t, err)

			for _, obj := range tt.initRuntimeObjs {
				err := cli.Create(ctx, obj)
				require.NoError(t, err)
			}

			if err := EnsureObject(ctx, cli, tt.args.obj, tt.args.applyOpts...); (err != nil) != tt.wantErr {
				t.Errorf("EnsureObject() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.assertObj != nil {
				tt.assertObj(t, tt.args.obj)
			}
		})
	}
}
