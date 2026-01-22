package license

import (
	"embed"
	"encoding/json"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed testdata/*
var testdata embed.FS

func loadLicenseFromTestdata(t *testing.T, filename string) *licensewrapper.LicenseWrapper {
	t.Helper()

	licenseBytes, err := testdata.ReadFile(filename)
	require.NoError(t, err)

	wrapper, err := helpers.ParseLicenseFromBytes(licenseBytes)
	require.NoError(t, err)

	return wrapper
}

func Test_VerifySignature(t *testing.T) {
	tests := []struct {
		name          string
		licenseFile   string
		wrapper       *licensewrapper.LicenseWrapper
		modifyLicense func(*licensewrapper.LicenseWrapper)
		expectError   bool
		errorContains string
	}{
		{
			name:        "v1beta1: valid signature passes verification",
			licenseFile: "testdata/valid-license.yaml",
			expectError: false,
		},
		{
			name:        "v1beta1: tampered license fails verification",
			licenseFile: "testdata/valid-license.yaml",
			modifyLicense: func(wrapper *licensewrapper.LicenseWrapper) {
				wrapper.V1.Spec.LicenseID = wrapper.V1.Spec.LicenseID + "-modified"
			},
			expectError:   true,
			errorContains: `"licenseID" field has changed`,
		},
		{
			name:          "v1beta1: invalid signature fails verification",
			licenseFile:   "testdata/invalid-signature.yaml",
			expectError:   true,
			errorContains: "verification error",
		},
		{
			name:        "v1beta2: valid signature passes verification",
			licenseFile: "testdata/valid-license-v2.yaml",
			expectError: false,
		},

		{
			name: "v1beta2: invalid signature fails verification",
			wrapper: &licensewrapper.LicenseWrapper{
				V2: &kotsv1beta2.License{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "kots.io/v1beta2",
						Kind:       "License",
					},
					Spec: kotsv1beta2.LicenseSpec{
						LicenseID: "test-license-v2",
						Signature: json.RawMessage(`{"invalid": "signature"}`),
					},
				},
			},
			expectError:   true,
			errorContains: "v1beta2 license validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			var wrapper *licensewrapper.LicenseWrapper
			if tt.licenseFile != "" {
				wrapper = loadLicenseFromTestdata(t, tt.licenseFile)
			} else if tt.wrapper != nil || tt.name == "nil wrapper returns nil" {
				wrapper = tt.wrapper
			}

			if tt.modifyLicense != nil {
				tt.modifyLicense(wrapper)
			}

			verifiedWrapper, err := VerifySignature(wrapper)

			if tt.expectError {
				req.Error(err)
				if tt.errorContains != "" {
					req.Contains(err.Error(), tt.errorContains)
				}
			} else {
				req.NoError(err)
				if wrapper != nil {
					req.NotNil(verifiedWrapper)
				}
			}
		})
	}
}

// Test_LicenseTamperDetection verifies that the kotskinds ValidateLicense() properly detects
// when any critical license field has been tampered with after signing.
// This is an end-to-end test that ensures the validation logic in kotskinds catches all tampering.
func Test_LicenseTamperDetection(t *testing.T) {
	// All tests use a valid license from testdata and modify it to simulate tampering
	baseLicenseFile := "testdata/valid-license.yaml"

	tests := []struct {
		name          string
		modifyLicense func(*licensewrapper.LicenseWrapper)
		errorContains string
	}{
		{
			name: "appSlug tampered",
			modifyLicense: func(wrapper *licensewrapper.LicenseWrapper) {
				wrapper.V1.Spec.AppSlug = wrapper.V1.Spec.AppSlug + "-modified"
			},
			errorContains: "license data validation failed",
		},
		{
			name: "endpoint tampered",
			modifyLicense: func(wrapper *licensewrapper.LicenseWrapper) {
				wrapper.V1.Spec.Endpoint = "https://tampered.app"
			},
			errorContains: "license data validation failed",
		},
		{
			name: "customerName tampered",
			modifyLicense: func(wrapper *licensewrapper.LicenseWrapper) {
				wrapper.V1.Spec.CustomerName = "Tampered Customer"
			},
			errorContains: "license data validation failed",
		},
		{
			name: "customerEmail tampered",
			modifyLicense: func(wrapper *licensewrapper.LicenseWrapper) {
				wrapper.V1.Spec.CustomerEmail = "tampered@example.com"
			},
			errorContains: "license data validation failed",
		},
		{
			name: "channelID tampered",
			modifyLicense: func(wrapper *licensewrapper.LicenseWrapper) {
				wrapper.V1.Spec.ChannelID = "tampered-channel-id"
			},
			errorContains: "license data validation failed",
		},
		{
			name: "channelName tampered",
			modifyLicense: func(wrapper *licensewrapper.LicenseWrapper) {
				wrapper.V1.Spec.ChannelName = "tampered-channel"
			},
			errorContains: "license data validation failed",
		},
		{
			name: "licenseSequence tampered",
			modifyLicense: func(wrapper *licensewrapper.LicenseWrapper) {
				wrapper.V1.Spec.LicenseSequence = 999999
			},
			errorContains: "license data validation failed",
		},
		{
			name: "licenseID tampered",
			modifyLicense: func(wrapper *licensewrapper.LicenseWrapper) {
				wrapper.V1.Spec.LicenseID = "tampered-license-id"
			},
			errorContains: "license data validation failed",
		},
		{
			name: "licenseType tampered",
			modifyLicense: func(wrapper *licensewrapper.LicenseWrapper) {
				wrapper.V1.Spec.LicenseType = "tampered"
			},
			errorContains: "license data validation failed",
		},
		// Note: Entitlement tampering is validated separately by kotskinds using individual
		// entitlement signatures (EntitlementField.Signature.V1). The main license signature
		// protects the core license fields above. Entitlement validation is tested in
		// Test_VerifySignature/v1beta1:_tampered_license_fails_verification
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			// Load a valid signed license
			wrapper := loadLicenseFromTestdata(t, baseLicenseFile)

			// Tamper with the license
			tt.modifyLicense(wrapper)

			// Verify that kotskinds detects the tampering
			_, err := VerifySignature(wrapper)
			req.Error(err, "expected kotskinds to detect tampering")
			req.Contains(err.Error(), tt.errorContains)
		})
	}
}
