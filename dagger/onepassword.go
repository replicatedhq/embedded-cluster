package main

import (
	"dagger/embedded-cluster/internal/dagger"
	"fmt"
)

// OnePassword holds configuration for 1Password secret retrieval.
type OnePassword struct {
	// +private
	VaultName string
	// +private
	ServiceAccount *dagger.Secret
	// +private
	ItemName string
}

// Retrieves a secret from 1Password by item and field name.
func (m *OnePassword) FindSecret(
	// Field name
	field string,
	// +optional
	// Section name
	section *string,
) *dagger.Secret {
	var opts []dagger.OnepasswordFindSecretOpts
	if section != nil {
		opts = append(opts, dagger.OnepasswordFindSecretOpts{
			Section: *section,
		})
	}
	return dag.Onepassword().FindSecret(m.ServiceAccount, m.VaultName, m.ItemName, field, opts...)
}

// Configures 1Password integration for retrieving secrets.
//
// This enables other Dagger functions to automatically retrieve secrets from 1Password
// by calling mustResolveSecret or resolveSecret. The service account token is used to
// authenticate with 1Password and access secrets in the specified vault.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  test-provision-vm
func (m *EmbeddedCluster) WithOnePassword(
	serviceAccount *dagger.Secret,
	// +default="Developer Automation"
	vaultName string,
	// +default="EC Dev"
	itemName string,
) *EmbeddedCluster {
	m.OnePassword = &OnePassword{
		ServiceAccount: serviceAccount,
		VaultName:      vaultName,
		ItemName:       itemName,
	}
	return m
}

// mustResolveSecret resolves a secret from either a credential override or 1Password.
//
// If credentialOverride is provided, it returns that secret immediately.
// Otherwise, it attempts to retrieve the secret from 1Password using the item and field names.
// Panics if no secret is found from either source.
func (m *EmbeddedCluster) mustResolveSecret(credentialOverride *dagger.Secret, fieldName string) *dagger.Secret {
	secret := m.resolveSecret(credentialOverride, fieldName)
	if secret == nil {
		panic(fmt.Errorf("no secret passed as arg nor found in 1password for field %q", fieldName))
	}
	return secret
}

// resolveSecret resolves a secret from either a credential override or 1Password.
//
// If credentialOverride is provided, it returns that secret immediately.
// Otherwise, it attempts to retrieve the secret from 1Password using the item and field names.
// Returns nil if no secret is found from either source.
func (m *EmbeddedCluster) resolveSecret(credentialOverride *dagger.Secret, fieldName string) *dagger.Secret {
	if credentialOverride != nil {
		return credentialOverride
	}

	if m.OnePassword == nil {
		return nil
	}

	return m.OnePassword.FindSecret(
		fieldName,
		nil,
	)
}
