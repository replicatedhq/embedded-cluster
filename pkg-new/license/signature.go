package license

import (
	"fmt"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
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

	"6f21b4d9865f45b8a15bd884fb4028d2": []byte(`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwWEoVA/AQhzgG81k4V+C
7c7xoNKSnP8XKSkuYiCbsYyicsWxMtwExkueVKXvEa/DQm7NCDBOdFQFhFQKzKvn
Jh2rXnPZn3OyNQ9Ru+4XBi4kOa1V9g5VFSgwbBttuVtWtPZC2B4vdCVXyX4TzLYe
c0rGbq+obBb4RNKBBGTdoWy+IHlObc5QOpEzubUmJ1VqmCTUyduKeOn24b+TvcmJ
i5PY1r8iKGhJJOAPt4KjBlIj67uqcGq3N9RA8pHQjn0ZXsfiLOmCeR6kFHbnNr4n
L7HvoEDR12K2Ci4+n7A/EAowHI/ZywcM7wADcWx4tOERPz0Pm2SUvVCjPVPc0xdN
KwIDAQAB
-----END PUBLIC KEY-----`), // Dryrun (test-only, private key in tests/dryrun/assets)
}

// VerifySignature verifies the cryptographic signature of a license wrapper.
// It handles both v1beta1 and v1beta2 licenses by delegating to their ValidateLicense methods.
// Returns the wrapper unchanged if validation succeeds, or an error if validation fails.
func VerifySignature(wrapper *licensewrapper.LicenseWrapper) (*licensewrapper.LicenseWrapper, error) {
	if wrapper.IsV1() {
		_, err := wrapper.V1.ValidateLicense()
		if err != nil {
			return nil, fmt.Errorf("v1beta1 license validation failed: %w", err)
		}
		return wrapper, nil
	}

	if wrapper.IsV2() {
		_, err := wrapper.V2.ValidateLicense()
		if err != nil {
			return nil, fmt.Errorf("v1beta2 license validation failed: %w", err)
		}
		return wrapper, nil
	}

	return wrapper, nil
}
