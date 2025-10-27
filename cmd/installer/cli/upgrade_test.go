package cli

import (
	"context"
	"crypto/tls"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testCertData = `-----BEGIN CERTIFICATE-----
MIIDizCCAnOgAwIBAgIUJaAILNY7l9MR4mfMP4WiUObo6TIwDQYJKoZIhvcNAQEL
BQAwVTELMAkGA1UEBhMCVVMxDTALBgNVBAgMBFRlc3QxDTALBgNVBAcMBFRlc3Qx
DTALBgNVBAoMBFRlc3QxGTAXBgNVBAMMEHRlc3QuZXhhbXBsZS5jb20wHhcNMjUw
ODE5MTcwNTU4WhcNMjYwODE5MTcwNTU4WjBVMQswCQYDVQQGEwJVUzENMAsGA1UE
CAwEVGVzdDENMAsGA1UEBwwEVGVzdDENMAsGA1UECgwEVGVzdDEZMBcGA1UEAwwQ
dGVzdC5leGFtcGxlLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AMhkRyxUJE4JLrTbqq/Etdvd2osmkZJA5GXCRkWcGLBppNNqO1v8K0zy5dV9jgno
gjeQD2nTqZ++vmzR3wPObeB6MJY+2SYtFHvnT3G9HR4DcSX3uHUOBDjbUsW0OT6z
weT3t3eTVqNIY96rZRHz9VYrdC4EPlWyfoYTCHceZey3AqSgHWnHIxVaATWT/LFQ
yvRRlEBNf7/M5NX0qis91wKgGwe6u+P/ebmT1cXURufM0jSAMUbDIqr73Qq5m6t4
fv6/8XKAiVpA1VcACvR79kTi6hYMls88ShHuYLJK175ZQfkeJx77TI/UebALL9CZ
SCI1B08SMZOsr9GQMOKNIl8CAwEAAaNTMFEwHQYDVR0OBBYEFCQWAH7mJ0w4Iehv
PL72t8GCJ90uMB8GA1UdIwQYMBaAFCQWAH7mJ0w4IehvPL72t8GCJ90uMA8GA1Ud
EwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEBAFfEICcE4eFZkRfjcEkvrJ3T
KmMikNP2nPXv3h5Ie0DpprejPkDyOWe+UJBanYwAf8xXVwRTmE5PqQhEik2zTBlN
N745Izq1cUYIlyt9GHHycx384osYHKkGE9lAPEvyftlc9hCLSu/FVQ3+8CGwGm9i
cFNYLx/qrKkJxT0Lohi7VCAf7+S9UWjIiLaETGlejm6kPNLRZ0VoxIPgUmqePXfp
6gY5FSIzvH1kZ+bPZ3nqsGyT1l7TsubeTPDDGhpKgIFzcJX9WeY//bI4q1SpU1Fl
koNnBhDuuJxjiafIFCz4qVlf0kmRrz4jeXGXym8IjxUq0EpMgxGuSIkguPKiwFQ=
-----END CERTIFICATE-----`

	testKeyData = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDIZEcsVCROCS60
26qvxLXb3dqLJpGSQORlwkZFnBiwaaTTajtb/CtM8uXVfY4J6II3kA9p06mfvr5s
0d8Dzm3gejCWPtkmLRR7509xvR0eA3El97h1DgQ421LFtDk+s8Hk97d3k1ajSGPe
q2UR8/VWK3QuBD5Vsn6GEwh3HmXstwKkoB1pxyMVWgE1k/yxUMr0UZRATX+/zOTV
9KorPdcCoBsHurvj/3m5k9XF1EbnzNI0gDFGwyKq+90KuZureH7+v/FygIlaQNVX
AAr0e/ZE4uoWDJbPPEoR7mCySte+WUH5Hice+0yP1HmwCy/QmUgiNQdPEjGTrK/R
kDDijSJfAgMBAAECggEAHnl1g23GWaG22yU+110cZPPfrOKwJ6Q7t6fsRODAtm9S
dB5HKa13LkwQHL/rzmDwEKAVX/wi4xrAXc8q0areddFPO0IShuY7I76hC8R9PZe7
aNE72X1IshbUhyFpxTnUBkyPt50OA2XaXj4FcE3/5NtV3zug+SpcaGpTkr3qNS24
0Qf5X8AA1STec81c4BaXc8GgLsXz/4kWUSiwK0fjXcIpHkW28gtUyVmYu3FAPSdo
4bKdbqNUiYxF+JYLCQ9PyvFAqy7EhFLM4QkMICnSBNqNCPq3hVOr8K4V9luNnAmS
oU5gEHXmGM8a+kkdvLoZn3dO5tRk8ctV0vnLMYnXrQKBgQDl4/HDbv3oMiqS9nJK
+vQ7/yzLUb00fVzvWbvSLdEfGCgbRlDRKkNMgI5/BnFTJcbG5o3rIdBW37FY3iAy
p4iIm+VGiDz4lFApAQdiQXk9d2/mfB9ZVryUsKskvk6WTjom6+BRSvakqe2jIa/i
udnMFNGkJj6HzZqss1LKDiR5DQKBgQDfJqj5AlCyNUxjokWMH0BapuBVSHYZnxxD
xR5xX/5Q5fKDBpp4hMn8vFS4L8a5mCOBUPbuxEj7KY0Ho5bqYWmt+HyxP5TvDS9h
ZqgDdJuWdLB4hfzlUKekufFrpALvUT4AbmYdQ+ufkggU0mWGCfKaijlk4Hy/VRH7
w5ConbJWGwKBgADkF0XIoldKCnwzVFISEuxAmu3WzULs0XVkBaRU5SCXuWARr7J/
1W7weJzpa3sFBHY04ovsv5/2kftkMP/BQng1EnhpgsL74Cuog1zQICYq1lYwWPbB
rU1uOduUmT1f5D3OYDowbjBJMFCXitT4H235Dq7yLv/bviO5NjLuRxnpAoGBAJBj
LnA4jEhS7kOFiuSYkAZX9c2Y3jnD1wEOuZz4VNC5iMo46phSq3Np1JN87mPGSirx
XWWvAd3py8QGmK69KykTIHN7xX1MFb07NDlQKSAYDttdLv6dymtumQRiEjgRZEHZ
LR+AhCQy1CHM5T3uj9ho2awpCO6wN7uklaRUrUDDAoGBAK/EPsIxm5yj+kFIc/qk
SGwCw13pfbshh9hyU6O//h3czLnN9dgTllfsC7qqxsgrMCVZO9ZIfh5eb44+p7Id
r3glM4yhSJwf/cAWmt1A7DGOYnV7FF2wkDJJPX/Vag1uEsqrzwnAdFBymK5dwDsu
oxhVqyhpk86rf0rT5DcD/sBw
-----END PRIVATE KEY-----`
)

func Test_readPasswordHash(t *testing.T) {
	tests := []struct {
		name         string
		secret       *corev1.Secret
		wantErr      string
		wantPassword []byte
	}{
		{
			name: "valid password secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-password",
					Namespace: constants.KotsadmNamespace,
				},
				Data: map[string][]byte{
					"passwordBcrypt": []byte("$2a$10$hashedpassword"),
				},
			},
			wantPassword: []byte("$2a$10$hashedpassword"),
		},
		{
			name: "secret missing passwordBcrypt data",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-password",
					Namespace: constants.KotsadmNamespace,
				},
				Data: map[string][]byte{
					"otherField": []byte("somevalue"),
				},
			},
			wantErr: "kotsadm-password secret is missing required passwordBcrypt data",
		},
		{
			name:    "secret not found",
			secret:  nil,
			wantErr: "read kotsadm-password secret from cluster",
		},
		{
			name: "secret with empty passwordBcrypt",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-password",
					Namespace: constants.KotsadmNamespace,
				},
				Data: map[string][]byte{
					"passwordBcrypt": []byte(""),
				},
			},
			wantPassword: []byte(""),
		},
		{
			name: "secret with nil data",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-password",
					Namespace: constants.KotsadmNamespace,
				},
				Data: nil,
			},
			wantErr: "kotsadm-password secret is missing required passwordBcrypt data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			// Create a fake Kubernetes client
			scheme := runtime.NewScheme()
			err := corev1.AddToScheme(scheme)
			req.NoError(err)

			var objects []client.Object
			if tt.secret != nil {
				objects = append(objects, tt.secret)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			passwordHash, err := readPasswordHash(context.Background(), fakeClient)

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
				req.Nil(passwordHash)
			} else {
				req.NoError(err)
				req.Equal(tt.wantPassword, passwordHash)
			}
		})
	}
}

func Test_getClusterID(t *testing.T) {
	tests := []struct {
		name          string
		installation  *ecv1beta1.Installation
		wantErr       string
		wantClusterID string
	}{
		{
			name: "valid installation with cluster ID",
			installation: &ecv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-installation",
				},
				Spec: ecv1beta1.InstallationSpec{
					ClusterID: "test-cluster-id-123",
				},
			},
			wantClusterID: "test-cluster-id-123",
		},
		{
			name: "installation with empty cluster ID",
			installation: &ecv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-installation",
				},
				Spec: ecv1beta1.InstallationSpec{
					ClusterID: "",
				},
			},
			wantErr: "existing installation has empty cluster ID",
		},
		{
			name:         "no installation found",
			installation: nil,
			wantErr:      "no installations found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			// Create a fake Kubernetes client
			scheme := runtime.NewScheme()
			err := ecv1beta1.AddToScheme(scheme)
			req.NoError(err)

			var objects []client.Object
			if tt.installation != nil {
				objects = append(objects, tt.installation)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			clusterID, err := getClusterID(context.Background(), fakeClient)

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
				req.Empty(clusterID)
			} else {
				req.NoError(err)
				req.Equal(tt.wantClusterID, clusterID)
			}
		})
	}
}

func Test_readTLSConfig(t *testing.T) {
	// Test certificate data

	tests := []struct {
		name         string
		secret       *corev1.Secret
		wantErr      string
		wantTLS      bool
		wantHostname string
	}{
		{
			name: "valid TLS secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-tls",
					Namespace: constants.KotsadmNamespace,
				},
				Data: map[string][]byte{
					"tls.crt": []byte(testCertData),
					"tls.key": []byte(testKeyData),
				},
				StringData: map[string]string{
					"hostname": "example.com",
				},
			},
			wantTLS:      true,
			wantHostname: "example.com",
		},
		{
			name: "secret missing tls.crt",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-tls",
					Namespace: constants.KotsadmNamespace,
				},
				Data: map[string][]byte{
					"tls.key": []byte(testKeyData),
				},
			},
			wantErr: "kotsadm-tls secret is missing required tls.crt or tls.key data",
		},
		{
			name: "secret missing tls.key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-tls",
					Namespace: constants.KotsadmNamespace,
				},
				Data: map[string][]byte{
					"tls.crt": []byte(testCertData),
				},
			},
			wantErr: "kotsadm-tls secret is missing required tls.crt or tls.key data",
		},
		{
			name: "secret with empty certificate data",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-tls",
					Namespace: constants.KotsadmNamespace,
				},
				Data: map[string][]byte{
					"tls.crt": []byte(""),
					"tls.key": []byte(""),
				},
			},
			wantErr: "kotsadm-tls secret is missing required tls.crt or tls.key data",
		},
		{
			name:    "secret not found",
			secret:  nil,
			wantErr: "read kotsadm-tls secret from cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			// Create a fake Kubernetes client
			scheme := runtime.NewScheme()
			err := corev1.AddToScheme(scheme)
			req.NoError(err)

			var objects []client.Object
			if tt.secret != nil {
				objects = append(objects, tt.secret)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			tlsConfig, err := readTLSConfig(context.Background(), fakeClient)

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
				req.Empty(tlsConfig.CertBytes)
				req.Empty(tlsConfig.KeyBytes)
			} else {
				req.NoError(err)
				if tt.wantTLS {
					req.NotEmpty(tlsConfig.CertBytes)
					req.NotEmpty(tlsConfig.KeyBytes)
					req.Equal(tt.wantHostname, tlsConfig.Hostname)

					// Verify the certificate is actually valid
					_, err := tls.X509KeyPair(tlsConfig.CertBytes, tlsConfig.KeyBytes)
					req.NoError(err)
				}
			}
		})
	}
}

func Test_preRunUpgrade(t *testing.T) {
	tests := []struct {
		name          string
		flags         UpgradeCmdFlags
		installation  *ecv1beta1.Installation
		dataDir       string
		wantErr       string
		wantClusterID string
	}{
		{
			name: "no existing installation",
			flags: UpgradeCmdFlags{
				target: "linux",
			},
			installation: nil,
			wantErr:      "failed to get existing installation",
		},
		{
			name: "installation missing cluster ID",
			flags: UpgradeCmdFlags{
				target: "linux",
			},
			installation: &ecv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-installation",
				},
				Spec: ecv1beta1.InstallationSpec{
					ClusterID: "",
				},
			},
			wantErr: "existing installation has empty cluster ID",
		},
		{
			name: "missing data directory",
			flags: UpgradeCmdFlags{
				target: "linux",
			},
			installation: &ecv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-installation",
				},
				Spec: ecv1beta1.InstallationSpec{
					ClusterID: "test-cluster-123",
				},
			},
			dataDir:       "/nonexistent/path",
			wantClusterID: "test-cluster-123",
			wantErr:       "failed to stat data directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			// Create temporary data directory
			tmpDir, err := helpers.MkdirTemp("", "prerunupgrade-test-*")
			req.NoError(err)
			defer helpers.RemoveAll(tmpDir)

			// Set up data directory
			dataDir := tt.dataDir
			if dataDir == "" {
				dataDir = tmpDir
			}

			// Create fake Kubernetes client
			scheme := runtime.NewScheme()
			err = corev1.AddToScheme(scheme)
			req.NoError(err)
			err = ecv1beta1.AddToScheme(scheme)
			req.NoError(err)

			var objects []client.Object
			if tt.installation != nil {
				objects = append(objects, tt.installation)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			// Create mock runtime config
			mockRC := &runtimeconfig.MockRuntimeConfig{}
			mockRC.On("EmbeddedClusterHomeDirectory").Return(dataDir)
			mockRC.On("ManagerPort").Return(8800)

			// Create upgrade config
			upgradeConfig := &upgradeConfig{}

			err = preRunUpgrade(context.Background(), tt.flags, upgradeConfig, mockRC, fakeClient, "test-app")

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
				if tt.wantClusterID != "" {
					req.Equal(tt.wantClusterID, upgradeConfig.clusterID)
				}
			} else {
				req.NoError(err)
				if tt.wantClusterID != "" {
					req.Equal(tt.wantClusterID, upgradeConfig.clusterID)
				}
			}
		})
	}
}
