// Package ssh handles SSH configuration for the local machine. Things
// like creating keys and adding them to the authorized_keys file are
// handled here.
package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"

	"github.com/replicatedhq/helmvm/pkg/defaults"
)

// AllowLocalSSH configures the local machine to allow SSH access to the
// machine from the local machine. Sets up a new SSH keypair and adds the
// public key to the authorized_keys file.
func AllowLocalSSH() error {
	kpath := defaults.SSHKeyPath()
	if _, err := os.Stat(kpath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("unable to read ssh key: %w", err)
	}
	privkey, err := createPrivateKey()
	if err != nil {
		return fmt.Errorf("unable to create private key: %w", err)
	}
	pubkey, err := encodePublicKey(&privkey.PublicKey)
	if err != nil {
		return fmt.Errorf("unable to encode public key: %w", err)
	}
	if err := updateAuthorizedKeys(pubkey); err != nil {
		return fmt.Errorf("unable to update authorized keys: %w", err)
	}
	return nil
}

// updateAuthorizedKeys adds the given public key to the authorized_keys file.
func updateAuthorizedKeys(pubkey []byte) error {
	path := defaults.SSHAuthorizedKeysPath()
	fp, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("unable to open authorized keys file: %w", err)
	}
	defer fp.Close()
	pubkey = append(pubkey, '\n')
	if _, err := fp.Write(pubkey); err != nil {
		return fmt.Errorf("unable to write authorized keys file: %w", err)
	}
	return nil
}

// encodePublicKey encodes the given public key into the OpenSSH format. This
// function writes the public key to the right place and also returns it.
func encodePublicKey(privkey *rsa.PublicKey) ([]byte, error) {
	publicRsaKey, err := ssh.NewPublicKey(privkey)
	if err != nil {
		return nil, fmt.Errorf("unable to generate public key: %w", err)
	}
	pubkeyb := ssh.MarshalAuthorizedKey(publicRsaKey)
	pubpath := fmt.Sprintf("%s.pub", defaults.SSHKeyPath())
	fp, err := os.Create(pubpath)
	if err != nil {
		return nil, fmt.Errorf("unable to create public key file: %w", err)
	}
	defer fp.Close()
	if _, err := fp.Write(pubkeyb); err != nil {
		return nil, fmt.Errorf("unable to write public key file: %w", err)
	}
	return pubkeyb, nil
}

// createPrivateKey creates a new RSA private key. Writes it to the right place
// and also returns it.
func createPrivateKey() (*rsa.PrivateKey, error) {
	privkey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("unable to generate rsa key: %w", err)
	}
	privpem := &pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privkey),
	}
	path := defaults.SSHKeyPath()
	fp, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("unable to create private key file: %w", err)
	}
	defer fp.Close()
	if err := pem.Encode(fp, privpem); err != nil {
		return nil, fmt.Errorf("unable to write private ssh file: %w", err)
	}
	return privkey, nil
}
