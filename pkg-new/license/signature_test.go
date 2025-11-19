package license

import (
	"embed"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed testdata/*
var testdata embed.FS

func loadLicenseFromTestdata(t *testing.T, filename string) *kotsv1beta1.License {
	t.Helper()

	licenseBytes, err := testdata.ReadFile(filename)
	require.NoError(t, err)

	license, err := helpers.ParseLicenseFromBytes(licenseBytes)
	require.NoError(t, err)

	return license
}

func Test_VerifySignature(t *testing.T) {
	tests := []struct {
		name          string
		licenseFile   string
		modifyLicense func(*kotsv1beta1.License)
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid signature passes verification",
			licenseFile: "testdata/valid-license.yaml",
			expectError: false,
		},
		{
			name:        "tampered license fails verification",
			licenseFile: "testdata/valid-license.yaml",
			modifyLicense: func(license *kotsv1beta1.License) {
				license.Spec.LicenseID = license.Spec.LicenseID + "-modified"
			},
			expectError:   true,
			errorContains: `"licenseID" field has changed`,
		},
		{
			name:          "invalid signature fails verification",
			licenseFile:   "testdata/invalid-signature.yaml",
			expectError:   true,
			errorContains: "signature is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			license := loadLicenseFromTestdata(t, tt.licenseFile)

			if tt.modifyLicense != nil {
				tt.modifyLicense(license)
			}

			verifiedLicense, err := VerifySignature(license)

			if tt.expectError {
				req.Error(err)
				if tt.errorContains != "" {
					req.Contains(err.Error(), tt.errorContains)
				}
			} else {
				req.NoError(err)
				req.NotNil(verifiedLicense)
			}
		})
	}
}

func Test_verifyLicenseData(t *testing.T) {
	// Create a base license to use for all tests
	baseLicense := &kotsv1beta1.License{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kots.io/v1beta1",
			Kind:       "License",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-license",
		},
		Spec: kotsv1beta1.LicenseSpec{
			AppSlug:                        "test-app-slug",
			Endpoint:                       "https://replicated.app",
			CustomerName:                   "Test Customer",
			CustomerEmail:                  "test@example.com",
			ChannelID:                      "test-channel-id",
			ChannelName:                    "test-channel",
			LicenseSequence:                42,
			LicenseID:                      "test-license-id",
			LicenseType:                    "prod",
			IsAirgapSupported:              true,
			IsGitOpsSupported:              false,
			IsIdentityServiceSupported:     true,
			IsGeoaxisSupported:             false,
			IsSnapshotSupported:            true,
			IsDisasterRecoverySupported:    true,
			IsSupportBundleUploadSupported: true,
			IsSemverRequired:               false,
			Entitlements: map[string]kotsv1beta1.EntitlementField{
				"expires_at": {
					Title:       "Expiration",
					Description: "License Expiration",
					Value:       kotsv1beta1.EntitlementValue{Type: kotsv1beta1.String, StrVal: "2025-12-31"},
					ValueType:   "String",
				},
			},
		},
	}

	tests := []struct {
		name       string
		outer      *kotsv1beta1.License
		inner      *kotsv1beta1.License
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "happy path - all fields match",
			outer:   baseLicense.DeepCopy(),
			inner:   baseLicense.DeepCopy(),
			wantErr: false,
		},
		{
			name: "appSlug changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.AppSlug = "modified-app-slug"
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"appSlug" field has changed to "modified-app-slug" (license) from "test-app-slug" (within signature)`,
		},
		{
			name: "endpoint changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.Endpoint = "https://modified.app"
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"endpoint" field has changed to "https://modified.app" (license) from "https://replicated.app" (within signature)`,
		},
		{
			name: "customerName changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.CustomerName = "Modified Customer"
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"CustomerName" field has changed to "Modified Customer" (license) from "Test Customer" (within signature)`,
		},
		{
			name: "customerEmail changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.CustomerEmail = "modified@example.com"
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"CustomerEmail" field has changed to "modified@example.com" (license) from "test@example.com" (within signature)`,
		},
		{
			name: "channelID changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.ChannelID = "modified-channel-id"
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"channelID" field has changed to "modified-channel-id" (license) from "test-channel-id" (within signature)`,
		},
		{
			name: "channelName changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.ChannelName = "modified-channel"
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"channelName" field has changed to "modified-channel" (license) from "test-channel" (within signature)`,
		},
		{
			name: "licenseSequence changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.LicenseSequence = 99
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"licenseSequence" field has changed`,
		},
		{
			name: "licenseID changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.LicenseID = "modified-license-id"
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"licenseID" field has changed to "modified-license-id" (license) from "test-license-id" (within signature)`,
		},
		{
			name: "licenseType changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.LicenseType = "dev"
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"LicenseType" field has changed to "dev" (license) from "prod" (within signature)`,
		},
		{
			name: "isAirgapSupported changed from true to false",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.IsAirgapSupported = false
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"IsAirgapSupported" field has changed to false (license) from true (within signature)`,
		},
		{
			name: "isGitOpsSupported changed from false to true",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.IsGitOpsSupported = true
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"IsGitOpsSupported" field has changed to true (license) from false (within signature)`,
		},
		{
			name: "isIdentityServiceSupported changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.IsIdentityServiceSupported = false
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"IsIdentityServiceSupported" field has changed to false (license) from true (within signature)`,
		},
		{
			name: "isGeoaxisSupported changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.IsGeoaxisSupported = true
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"IsGeoaxisSupported" field has changed to true (license) from false (within signature)`,
		},
		{
			name: "isSnapshotSupported changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.IsSnapshotSupported = false
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"IsSnapshotSupported" field has changed to false (license) from true (within signature)`,
		},
		{
			name: "isDisasterRecoverySupported changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.IsDisasterRecoverySupported = false
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"IsDisasterRecoverySupported" field has changed to false (license) from true (within signature)`,
		},
		{
			name: "isSupportBundleUploadSupported changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.IsSupportBundleUploadSupported = false
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"IsSupportBundleUploadSupported" field has changed to false (license) from true (within signature)`,
		},
		{
			name: "isSemverRequired changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.IsSemverRequired = true
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"IsSemverRequired" field has changed to true (license) from false (within signature)`,
		},
		{
			name: "entitlements - different lengths",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.Entitlements["new_entitlement"] = kotsv1beta1.EntitlementField{
					Title: "New Entitlement",
					Value: kotsv1beta1.EntitlementValue{Type: kotsv1beta1.String, StrVal: "value"},
				}
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `"entitlements" field length has changed to 2 (license) from 1 (within signature)`,
		},
		{
			name: "entitlements - value changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.Entitlements["expires_at"] = kotsv1beta1.EntitlementField{
					Title:       "Expiration",
					Description: "License Expiration",
					Value:       kotsv1beta1.EntitlementValue{Type: kotsv1beta1.String, StrVal: "2026-12-31"},
					ValueType:   "String",
				}
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `entitlement "expires_at" value has changed to "2026-12-31" (license) from "2025-12-31" (within signature)`,
		},
		{
			name: "entitlements - title changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.Entitlements["expires_at"] = kotsv1beta1.EntitlementField{
					Title:       "Modified Expiration",
					Description: "License Expiration",
					Value:       kotsv1beta1.EntitlementValue{Type: kotsv1beta1.String, StrVal: "2025-12-31"},
					ValueType:   "String",
				}
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `entitlement "expires_at" title has changed to "Modified Expiration" (license) from "Expiration" (within signature)`,
		},
		{
			name: "entitlements - description changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.Entitlements["expires_at"] = kotsv1beta1.EntitlementField{
					Title:       "Expiration",
					Description: "Modified Description",
					Value:       kotsv1beta1.EntitlementValue{Type: kotsv1beta1.String, StrVal: "2025-12-31"},
					ValueType:   "String",
				}
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `entitlement "expires_at" description has changed to "Modified Description" (license) from "License Expiration" (within signature)`,
		},
		{
			name: "entitlements - hidden changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.Entitlements["expires_at"] = kotsv1beta1.EntitlementField{
					Title:       "Expiration",
					Description: "License Expiration",
					Value:       kotsv1beta1.EntitlementValue{Type: kotsv1beta1.String, StrVal: "2025-12-31"},
					ValueType:   "String",
					IsHidden:    true,
				}
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `entitlement "expires_at" hidden has changed to true (license) from false (within signature)`,
		},
		{
			name: "entitlements - valueType changed",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.Entitlements["expires_at"] = kotsv1beta1.EntitlementField{
					Title:       "Expiration",
					Description: "License Expiration",
					Value:       kotsv1beta1.EntitlementValue{Type: kotsv1beta1.String, StrVal: "2025-12-31"},
					ValueType:   "Integer",
				}
				return l
			}(),
			inner:      baseLicense.DeepCopy(),
			wantErr:    true,
			wantErrMsg: `entitlement "expires_at" value type has changed to "Integer" (license) from "String" (within signature)`,
		},
		{
			name: "entitlements - missing entitlement in inner",
			outer: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.Entitlements["new_key"] = kotsv1beta1.EntitlementField{
					Title: "New",
					Value: kotsv1beta1.EntitlementValue{Type: kotsv1beta1.String, StrVal: "value"},
				}
				return l
			}(),
			inner: func() *kotsv1beta1.License {
				l := baseLicense.DeepCopy()
				l.Spec.Entitlements = map[string]kotsv1beta1.EntitlementField{} // empty entitlements
				return l
			}(),
			wantErr:    true,
			wantErrMsg: `"entitlements" field length has changed`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			err := verifyLicenseData(tt.outer, tt.inner)
			if tt.wantErr {
				req.Error(err)
				if tt.wantErrMsg != "" {
					req.Contains(err.Error(), tt.wantErrMsg)
				}
			} else {
				req.NoError(err)
			}
		})
	}
}
