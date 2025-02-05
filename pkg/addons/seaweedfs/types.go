package seaweedfs

type seaweedfsConfig struct {
	Identities []seaweedfsIdentity `json:"identities"`
}

type seaweedfsIdentity struct {
	Name        string                        `json:"name"`
	Credentials []seaweedfsIdentityCredential `json:"credentials"`
	Actions     []string                      `json:"actions"`
}

type seaweedfsIdentityCredential struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}
