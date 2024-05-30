package registry

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

func (c seaweedfsConfig) getCredentials(name string) (seaweedfsIdentityCredential, bool) {
	for _, identity := range c.Identities {
		if identity.Name == name {
			if len(identity.Credentials) == 0 {
				return seaweedfsIdentityCredential{}, false
			}
			return identity.Credentials[0], true
		}
	}
	return seaweedfsIdentityCredential{}, false
}
