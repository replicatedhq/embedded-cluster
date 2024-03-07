package airgap

import (
	"context"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestCreateAppConfigMaps(t *testing.T) {
	releaseData := `# channel release object
channelID: "testID"
appSlug: "test-app-slug"
versionLabel: "test-version-label"`
	err := release.SetReleaseDataForTests(map[string][]byte{
		"release.yaml": []byte(releaseData),
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		airgapFile     string
		wantConfigmaps []corev1.ConfigMap
	}{
		{
			name:       "tiny-airgap-noimages",
			airgapFile: "tiny-airgap-noimages.airgap",
			wantConfigmaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-airgap-meta",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"kots.io/app":        "test-app-slug",
							"kots.io/automation": "airgap",
							"kots.io/kotsadm":    "true",
						},
					},
					Data: map[string]string{
						"airgap.yaml": "YXBpVmVyc2lvbjoga290cy5pby92MWJldGExCmtpbmQ6IEFpcmdhcAptZXRhZGF0YToKICBjcmVhdGlvblRpbWVzdGFtcDogbnVsbApzcGVjOgogIGFwcFNsdWc6IGxhdmVyeWEtdGlueS1haXJnYXAKICBjaGFubmVsSUQ6IDJkTXJBcUpqclB6ZmVOSHY5YmMwZ0NIaDI1TgogIGNoYW5uZWxOYW1lOiBTdGFibGUKICBmb3JtYXQ6IGRvY2tlci1hcmNoaXZlCiAgcmVwbGljYXRlZENoYXJ0TmFtZXM6CiAgLSByZXBsaWNhdGVkCiAgLSByZXBsaWNhdGVkLXNkawogIHNhdmVkSW1hZ2VzOgogIC0gYWxwaW5lOjMuMTkuMQogIHNpZ25hdHVyZTogUFE0WnM0ZTRnMXNncmQxbFlvZzJpMjMraXhiRFhYM05hbmNPY0RkSytKcUQxUzRlbG1rSGhzR0lVYXpJbDE1ckw0WXVKUWR6ZWVtMGdlSzE0UEtBRE4rMFlMenZFVm05R3cxQ29xK3kzWkRwVW4yK09uN2NhSzRrMXZja0FFYm9tVUR3N0NtNUFHeFlERlBpejRpQytPbkttRllkZlU4RnFTTlQwaU1VeGpUdkJMZGxJZjlWT2g1d3NiaTVKNTExVUNFdjJIdDlVZXhjTkdvYmdvbHJDNUFVV0tBdmJING1HeG5TWFZSU0hqWHRzQUphSXcvQXcwUmRRODMwQUlhVEV6K0wrcWJnd2FzUUc3bEV4a2FRejJkRWJ5d1BQMm5MOVppeUNPSUFzamFLaWNsR3g4SHpLRENrN1dvbXQ0K1dPZnVzcXlrNm1VRmUvRWZsWC9sMlRBPT0KICB1cGRhdGVDdXJzb3I6ICIxIgogIHZlcnNpb25MYWJlbDogMC4xLjAKc3RhdHVzOiB7fQo=",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-airgap-cluster-config-yaml",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"kots.io/app":        "test-app-slug",
							"kots.io/automation": "airgap",
							"kots.io/kotsadm":    "true",
						},
					},
					Data: map[string]string{
						"cluster-config.yaml": "YXBpVmVyc2lvbjogZW1iZWRkZWRjbHVzdGVyLnJlcGxpY2F0ZWQuY29tL3YxYmV0YTEKa2luZDogQ29uZmlnCm1ldGFkYXRhOgogIG5hbWU6ICJ0ZXN0LWNsdXN0ZXItY29uZmlnIgogIGFubm90YXRpb25zOgogICAga290cy5pby9leGNsdWRlOiAidHJ1ZSIKc3BlYzoKICB2ZXJzaW9uOiAiMS4yOS4yK2VjLjAiCiAgcm9sZXM6CiAgICBjb250cm9sbGVyOgogICAgICBsYWJlbHM6CiAgICAgICAgY29udHJvbGxlci10ZXN0LWxhYmVsOiBjb250cm9sbGVyLXRlc3QtbGFiZWwtdmFsdWUKICAgICAgbmFtZTogY29udHJvbGxlci1ub2RlCiAgICBjdXN0b206CiAgICAgIC0gbGFiZWxzOgogICAgICAgICAgYWJjLXRlc3QtbGFiZWw6IGFiYy10ZXN0LWxhYmVsLXZhbHVlCiAgICAgICAgICBhYmMtdGVzdC1sYWJlbC10d286IGFiYy10ZXN0LWxhYmVsLXZhbHVlLTIKICAgICAgICBuYW1lOiB3ZWIKICAgICAgLSBsYWJlbHM6CiAgICAgICAgICB4eXotdGVzdC1sYWJlbDogeHl6LXZhbHVlCiAgICAgICAgbmFtZTogYmFja2VuZAogICAgICAtIGxhYmVsczoKICAgICAgICAgIGVsYXN0aWNzZWFyY2gtbm9kZS1yb2xlOiBtYWluCiAgICAgICAgbmFtZTogZWxhc3RpY3NlYXJjaC1tYWluCiAgdW5zdXBwb3J0ZWRPdmVycmlkZXM6CiAgICBrMHM6IHwKICAgICAgY29uZmlnOgogICAgICAgIHNwZWM6CiAgICAgICAgICBhcGk6CiAgICAgICAgICAgIGV4dHJhQXJnczoKICAgICAgICAgICAgICBzZXJ2aWNlLW5vZGUtcG9ydC1yYW5nZTogODAtMzI3NjcKICBleHRlbnNpb25zOgogICAgaGVsbToKICAgICAgcmVwb3NpdG9yaWVzOgogICAgICAgIC0gbmFtZTogaW5ncmVzcy1uZ2lueAogICAgICAgICAgdXJsOiBodHRwczovL2t1YmVybmV0ZXMuZ2l0aHViLmlvL2luZ3Jlc3MtbmdpbngKICAgICAgY2hhcnRzOgogICAgICAgIC0gbmFtZTogaW5ncmVzcy1uZ2lueAogICAgICAgICAgY2hhcnRuYW1lOiBpbmdyZXNzLW5naW54L2luZ3Jlc3MtbmdpbngKICAgICAgICAgIG5hbWVzcGFjZTogaW5ncmVzcy1uZ2lueAogICAgICAgICAgdmVyc2lvbjogIjQuOS4xIgogICAgICAgICAgdmFsdWVzOiB8CiAgICAgICAgICAgIGNvbnRyb2xsZXI6CiAgICAgICAgICAgICAgc2VydmljZToKICAgICAgICAgICAgICAgIHR5cGU6IE5vZGVQb3J0CiAgICAgICAgICAgICAgICBub2RlUG9ydHM6CiAgICAgICAgICAgICAgICAgIGh0dHA6ICI4MCIKICAgICAgICAgICAgICAgICAgaHR0cHM6ICI0NDMiCiAgICAgICAgICAgICAgICBhbm5vdGF0aW9uczoKICAgICAgICAgICAgICAgICAgdGVzdC1jaGFydC1hbm5vdGF0aW9uOiB0ZXN0LWNoYXJ0LXZhbHVlCg==",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-airgap-example-deployment-yaml",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"kots.io/app":        "test-app-slug",
							"kots.io/automation": "airgap",
							"kots.io/kotsadm":    "true",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-airgap-k8s-app-yaml",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"kots.io/app":        "test-app-slug",
							"kots.io/automation": "airgap",
							"kots.io/kotsadm":    "true",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-airgap-kots-app-yaml",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"kots.io/app":        "test-app-slug",
							"kots.io/automation": "airgap",
							"kots.io/kotsadm":    "true",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-airgap-kots-config-yaml",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"kots.io/app":        "test-app-slug",
							"kots.io/automation": "airgap",
							"kots.io/kotsadm":    "true",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-airgap-kots-lint-config-yaml",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"kots.io/app":        "test-app-slug",
							"kots.io/automation": "airgap",
							"kots.io/kotsadm":    "true",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-airgap-kots-preflight-yaml",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"kots.io/app":        "test-app-slug",
							"kots.io/automation": "airgap",
							"kots.io/kotsadm":    "true",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-airgap-kots-support-bundle-yaml",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"kots.io/app":        "test-app-slug",
							"kots.io/automation": "airgap",
							"kots.io/kotsadm":    "true",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			ctx := context.Background()

			dir := ""
			dir, err = os.Getwd()
			req.NoError(err)
			t.Logf("Current working directory: %s", dir)

			// create fake client and run CreateAppConfigMaps
			fakeCLI := fake.NewClientBuilder().Build()
			err = CreateAppConfigMaps(ctx, fakeCLI, filepath.Join(dir, "testfiles", tt.airgapFile))
			req.NoError(err)

			// ensure that the configmaps created are the ones we expected
			allCms := &corev1.ConfigMapList{}
			err = fakeCLI.List(ctx, allCms, client.InNamespace("kotsadm"))
			req.NoError(err)
			req.Equal(len(tt.wantConfigmaps), len(allCms.Items))

			for _, cm := range tt.wantConfigmaps {
				gotCM := corev1.ConfigMap{}
				err = fakeCLI.Get(ctx, client.ObjectKey{Namespace: cm.Namespace, Name: cm.Name}, &gotCM)
				req.NoError(err)
				req.Equal(cm.ObjectMeta.Annotations, gotCM.ObjectMeta.Annotations)
				req.Equal(cm.ObjectMeta.Labels, gotCM.ObjectMeta.Labels)
				if cm.Data != nil {
					req.Equal(cm.Data, gotCM.Data)
				}
			}
		})
	}
}
