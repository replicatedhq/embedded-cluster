package cli

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

var (
	ErrSignatureInvalid = fmt.Errorf("signature is invalid")
	ErrSignatureMissing = fmt.Errorf("signature is missing")
)

type InnerSignature struct {
	LicenseSignature []byte `json:"licenseSignature"`
	PublicKey        string `json:"publicKey"`
	KeySignature     []byte `json:"keySignature"`
}

type OuterSignature struct {
	LicenseData    []byte `json:"licenseData"`
	InnerSignature []byte `json:"innerSignature"`
}

type KeySignature struct {
	Signature   []byte `json:"signature"`
	GlobalKeyId string `json:"globalKeyId"`
}

type LicenseDataError struct {
	message string
}

func (e LicenseDataError) Error() string {
	return e.message
}

// PublicKeys contains the RSA public keys used for license signature verification
var PublicKeys = map[string][]byte{
	"1d3f7f6b50714fe7b895554dd65773b0": []byte(`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAugyKfZV2gIDaY1Rzkjoo
fbNywGa04sGQIAqYwifMay2e2xzqRwswTRHQnr9SIWypkN86Cfn6QzOB8kkjERC1
DPNdsiKdjBFdcLaxxdyHgrXLgfdzhh6We+Lpq19JT5LCK3PXleZgt/a0aRBpIc1l
xKs57d8MTWUTVh3W3WYi6LbqAPScdmSiG7A145HhKXmmtZFEv4puE5dKmS5lkV2d
VU789XWrNFk74FKKHVwYMdppqAabB6cRBmU8YFiVEULOn+d1FtKRbO/vv/fbA9nX
PUG/1PgEQHogP+3cC4J7b7s9+kBmtHkpSq9x+OUu/5B+nT21dooS6adfQiI8iB/+
NQIDAQAB
-----END PUBLIC KEY-----`), // Dev

	"bdee56560cfb43c9b28bf98eacafa646": []byte(`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwdSHE8v64QH/yELBoPBl
GanhS3AD5vMAaqLLFnftwjmDKrxWwqNB9w1GVJWb5gVLvt/UlE/k+HVr5HFdomVI
TMvnvxhD0UvNyGFuUbXBMvQPPW9joR48LcCBLZl+RZTqR5HRhsIbujiExRDnteaq
mU1jG/oVlQkRoyOYrObTeoD0BdcZAr2PdGvgvJvpZduZtrKvjvsSJEBYExoPtko+
8AqhMBAI+qX1/SMix21qpmYSYLNeqN2Pplna0p2MK8yyaHY8KSqTF90ZJF1+P0ZF
MLt6S8/6PIX9WD+vFqmDpW1GCkB+p2OfxsYiAIX1ej98Ck3hoPQnOuiFIovV8aFQ
bQIDAQAB
-----END PUBLIC KEY-----`), // Production

	"de2c275656d04b1bb0f15cf70f0ea2a2": []byte(`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA2hHg1HER6NYlsqBs+B+B
txibtctT6YB5kxgE1sz7UmVnlcLs+Olc4OZJwD4vLsEU60SVW0HRoTfaGaradv0R
GUIxlFRSOnzjZEMkm/YKL3sdPQigi2m9O0P5tC9LQvzk49dFg5HJxiLODCgWwJ9g
q3pGs8OaAc0dop/tqUE7WqQfHLWJdTPP5pVDLDWybfAO4OmgVmx+oVXdCfMVlOzu
num6SOF+eBuERXQGbEfnd6eSRVokWhfMCfXNPTYtq14DaK9tvX4uzHsub+Asn6UN
OBIAESJntpZfdDDrNqbfOQYql2rqx1lJtU7lVFbTQTkKhj4teInEGO6FvLzy0UE9
swIDAQAB
-----END PUBLIC KEY-----`), // Staging
}

// verifySignature verifies the cryptographic signature of a license.
// It returns the verified license with the signature field populated, or an error if verification fails.
func verifySignature(license *kotsv1beta1.License) (*kotsv1beta1.License, error) {
	outerSignature := &OuterSignature{}
	if err := json.Unmarshal(license.Spec.Signature, outerSignature); err != nil {
		return nil, fmt.Errorf("failed to unmarshal license outer signature: %w", err)
	}

	isOldFormat := len(outerSignature.InnerSignature) == 0
	if isOldFormat {
		return verifyOldSignature(license)
	}

	innerSignature := &InnerSignature{}
	if err := json.Unmarshal(outerSignature.InnerSignature, innerSignature); err != nil {
		return nil, fmt.Errorf("failed to unmarshal license inner signature: %w", err)
	}

	keySignature := &KeySignature{}
	if err := json.Unmarshal(innerSignature.KeySignature, keySignature); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key signature: %w", err)
	}

	globalKeyPEM, ok := PublicKeys[keySignature.GlobalKeyId]
	if !ok {
		return nil, fmt.Errorf("unknown global key")
	}

	// verify that the app public key is properly signed with a replicated private key
	if err := verifyRSAPSS([]byte(innerSignature.PublicKey), keySignature.Signature, globalKeyPEM); err != nil {
		return nil, fmt.Errorf("failed to verify key signature: %w", err)
	}

	// verify that the license data is properly signed with the app private key
	if err := verifyRSAPSS(outerSignature.LicenseData, innerSignature.LicenseSignature, []byte(innerSignature.PublicKey)); err != nil {
		return nil, fmt.Errorf("failed to verify license signature: %w", err)
	}

	verifiedLicense := &kotsv1beta1.License{}
	if err := json.Unmarshal(outerSignature.LicenseData, verifiedLicense); err != nil {
		return nil, fmt.Errorf("failed to unmarshal license data: %w", err)
	}

	if err := verifyLicenseData(license, verifiedLicense); err != nil {
		return nil, LicenseDataError{message: err.Error()}
	}

	verifiedLicense.Spec.Signature = license.Spec.Signature

	return verifiedLicense, nil
}

// verifyRSAPSS verifies an RSA-PSS signature using MD5 hashing
func verifyRSAPSS(message, signature, publicKeyPEM []byte) error {
	pubBlock, _ := pem.Decode(publicKeyPEM)
	publicKey, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to load public key from PEM: %w", err)
	}

	var opts rsa.PSSOptions
	opts.SaltLength = rsa.PSSSaltLengthAuto

	newHash := crypto.MD5
	pssh := newHash.New()
	pssh.Write(message)
	hashed := pssh.Sum(nil)

	err = rsa.VerifyPSS(publicKey.(*rsa.PublicKey), newHash, hashed, signature, &opts)
	if err != nil {
		// this ordering makes errors.Cause a little more useful
		return fmt.Errorf("%w: %s", ErrSignatureInvalid, err.Error())
	}

	return nil
}

// verifyLicenseData ensures that critical license fields haven't been tampered with
// by comparing the outer license with the inner signed license
func verifyLicenseData(outerLicense *kotsv1beta1.License, innerLicense *kotsv1beta1.License) error {
	if outerLicense.Spec.AppSlug != innerLicense.Spec.AppSlug {
		return fmt.Errorf("\"appSlug\" field has changed to %q (license) from %q (within signature)", outerLicense.Spec.AppSlug, innerLicense.Spec.AppSlug)
	}
	if outerLicense.Spec.Endpoint != innerLicense.Spec.Endpoint {
		return fmt.Errorf("\"endpoint\" field has changed to %q (license) from %q (within signature)", outerLicense.Spec.Endpoint, innerLicense.Spec.Endpoint)
	}
	if outerLicense.Spec.CustomerName != innerLicense.Spec.CustomerName {
		return fmt.Errorf("\"CustomerName\" field has changed to %q (license) from %q (within signature)", outerLicense.Spec.CustomerName, innerLicense.Spec.CustomerName)
	}
	if outerLicense.Spec.CustomerEmail != innerLicense.Spec.CustomerEmail {
		return fmt.Errorf("\"CustomerEmail\" field has changed to %q (license) from %q (within signature)", outerLicense.Spec.CustomerEmail, innerLicense.Spec.CustomerEmail)
	}
	if outerLicense.Spec.ChannelID != innerLicense.Spec.ChannelID {
		return fmt.Errorf("\"channelID\" field has changed to %q (license) from %q (within signature)", outerLicense.Spec.ChannelID, innerLicense.Spec.ChannelID)
	}
	if outerLicense.Spec.ChannelName != innerLicense.Spec.ChannelName {
		return fmt.Errorf("\"channelName\" field has changed to %q (license) from %q (within signature)", outerLicense.Spec.ChannelName, innerLicense.Spec.ChannelName)
	}
	if outerLicense.Spec.LicenseSequence != innerLicense.Spec.LicenseSequence {
		return fmt.Errorf("\"licenseSequence\" field has changed to %q (license) from %q (within signature)", outerLicense.Spec.LicenseSequence, innerLicense.Spec.LicenseSequence)
	}
	if outerLicense.Spec.LicenseID != innerLicense.Spec.LicenseID {
		return fmt.Errorf("\"licenseID\" field has changed to %q (license) from %q (within signature)", outerLicense.Spec.LicenseID, innerLicense.Spec.LicenseID)
	}
	if outerLicense.Spec.LicenseType != innerLicense.Spec.LicenseType {
		return fmt.Errorf("\"LicenseType\" field has changed to %q (license) from %q (within signature)", outerLicense.Spec.LicenseType, innerLicense.Spec.LicenseType)
	}
	if outerLicense.Spec.IsAirgapSupported != innerLicense.Spec.IsAirgapSupported {
		return fmt.Errorf("\"IsAirgapSupported\" field has changed to %t (license) from %t (within signature)", outerLicense.Spec.IsAirgapSupported, innerLicense.Spec.IsAirgapSupported)
	}
	if outerLicense.Spec.IsGitOpsSupported != innerLicense.Spec.IsGitOpsSupported {
		return fmt.Errorf("\"IsGitOpsSupported\" field has changed to %t (license) from %t (within signature)", outerLicense.Spec.IsGitOpsSupported, innerLicense.Spec.IsGitOpsSupported)
	}
	if outerLicense.Spec.IsIdentityServiceSupported != innerLicense.Spec.IsIdentityServiceSupported {
		return fmt.Errorf("\"IsIdentityServiceSupported\" field has changed to %t (license) from %t (within signature)", outerLicense.Spec.IsIdentityServiceSupported, innerLicense.Spec.IsIdentityServiceSupported)
	}
	if outerLicense.Spec.IsGeoaxisSupported != innerLicense.Spec.IsGeoaxisSupported {
		return fmt.Errorf("\"IsGeoaxisSupported\" field has changed to %t (license) from %t (within signature)", outerLicense.Spec.IsGeoaxisSupported, innerLicense.Spec.IsGeoaxisSupported)
	}
	if outerLicense.Spec.IsSnapshotSupported != innerLicense.Spec.IsSnapshotSupported {
		return fmt.Errorf("\"IsSnapshotSupported\" field has changed to %t (license) from %t (within signature)", outerLicense.Spec.IsSnapshotSupported, innerLicense.Spec.IsSnapshotSupported)
	}
	if outerLicense.Spec.IsDisasterRecoverySupported != innerLicense.Spec.IsDisasterRecoverySupported {
		return fmt.Errorf("\"IsDisasterRecoverySupported\" field has changed to %t (license) from %t (within signature)", outerLicense.Spec.IsDisasterRecoverySupported, innerLicense.Spec.IsDisasterRecoverySupported)
	}
	if outerLicense.Spec.IsSupportBundleUploadSupported != innerLicense.Spec.IsSupportBundleUploadSupported {
		return fmt.Errorf("\"IsSupportBundleUploadSupported\" field has changed to %t (license) from %t (within signature)", outerLicense.Spec.IsSupportBundleUploadSupported, innerLicense.Spec.IsSupportBundleUploadSupported)
	}
	if outerLicense.Spec.IsSemverRequired != innerLicense.Spec.IsSemverRequired {
		return fmt.Errorf("\"IsSemverRequired\" field has changed to %t (license) from %t (within signature)", outerLicense.Spec.IsSemverRequired, innerLicense.Spec.IsSemverRequired)
	}

	// Check entitlements
	if len(outerLicense.Spec.Entitlements) != len(innerLicense.Spec.Entitlements) {
		return fmt.Errorf("\"entitlements\" field length has changed to %d (license) from %d (within signature)", len(outerLicense.Spec.Entitlements), len(innerLicense.Spec.Entitlements))
	}
	for k, outerEntitlement := range outerLicense.Spec.Entitlements {
		innerEntitlement, ok := innerLicense.Spec.Entitlements[k]
		if !ok {
			return fmt.Errorf("entitlement %q not found in the inner license", k)
		}
		if outerEntitlement.Value.Value() != innerEntitlement.Value.Value() {
			return fmt.Errorf("entitlement %q value has changed to %q (license) from %q (within signature)", k, outerEntitlement.Value.Value(), innerEntitlement.Value.Value())
		}
		if outerEntitlement.Title != innerEntitlement.Title {
			return fmt.Errorf("entitlement %q title has changed to %q (license) from %q (within signature)", k, outerEntitlement.Title, innerEntitlement.Title)
		}
		if outerEntitlement.Description != innerEntitlement.Description {
			return fmt.Errorf("entitlement %q description has changed to %q (license) from %q (within signature)", k, outerEntitlement.Description, innerEntitlement.Description)
		}
		if outerEntitlement.IsHidden != innerEntitlement.IsHidden {
			return fmt.Errorf("entitlement %q hidden has changed to %t (license) from %t (within signature)", k, outerEntitlement.IsHidden, innerEntitlement.IsHidden)
		}
		if outerEntitlement.ValueType != innerEntitlement.ValueType {
			return fmt.Errorf("entitlement %q value type has changed to %q (license) from %q (within signature)", k, outerEntitlement.ValueType, innerEntitlement.ValueType)
		}
	}

	return nil
}

// verifyOldSignature handles legacy license signature format verification
func verifyOldSignature(license *kotsv1beta1.License) (*kotsv1beta1.License, error) {
	signature := &InnerSignature{}
	if err := json.Unmarshal(license.Spec.Signature, signature); err != nil {
		// old licenses's signature is a single space character
		if len(license.Spec.Signature) == 0 || len(license.Spec.Signature) == 1 {
			return nil, ErrSignatureMissing
		}
		return nil, fmt.Errorf("failed to unmarshal license signature: %w", err)
	}

	keySignature := &KeySignature{}
	if err := json.Unmarshal(signature.KeySignature, keySignature); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key signature: %w", err)
	}

	globalKeyPEM, ok := PublicKeys[keySignature.GlobalKeyId]
	if !ok {
		return nil, fmt.Errorf("unknown global key")
	}

	if err := verifyRSAPSS([]byte(signature.PublicKey), keySignature.Signature, globalKeyPEM); err != nil {
		return nil, fmt.Errorf("failed to verify key signature: %w", err)
	}

	licenseMessage, err := getMessageFromLicense(license)
	if err != nil {
		return nil, fmt.Errorf("failed to convert license to message: %w", err)
	}

	if err := verifyRSAPSS(licenseMessage, signature.LicenseSignature, []byte(signature.PublicKey)); err != nil {
		return nil, fmt.Errorf("failed to verify license signature: %w", err)
	}

	return license, nil
}

// getMessageFromLicense creates a canonical message representation for old-format licenses
func getMessageFromLicense(license *kotsv1beta1.License) ([]byte, error) {
	// JSON marshaller will sort map keys automatically.
	fields := map[string]string{
		"apiVersion":             license.APIVersion,
		"kind":                   license.Kind,
		"metadata.name":          license.GetObjectMeta().GetName(),
		"spec.licenseID":         license.Spec.LicenseID,
		"spec.appSlug":           license.Spec.AppSlug,
		"spec.channelName":       license.Spec.ChannelName,
		"spec.endpoint":          license.Spec.Endpoint,
		"spec.isAirgapSupported": fmt.Sprintf("%t", license.Spec.IsAirgapSupported),
	}

	if license.Spec.LicenseSequence > 0 {
		fields["spec.licenseSequence"] = fmt.Sprintf("%d", license.Spec.LicenseSequence)
	}

	for k, v := range license.Spec.Entitlements {
		key := fmt.Sprintf("spec.entitlements.%s", k)
		val := map[string]string{
			"title":       v.Title,
			"description": v.Description,
			"value":       fmt.Sprintf("%v", v.Value.Value()),
		}
		valStr, err := json.Marshal(val)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal entitlement value: %s: %w", k, err)
		}
		fields[key] = string(valStr)
	}

	message, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message JSON: %w", err)
	}

	return message, err
}
