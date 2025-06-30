package types

type TLSConfig struct {
	CertBytes []byte
	KeyBytes  []byte
	Hostname  string
}
