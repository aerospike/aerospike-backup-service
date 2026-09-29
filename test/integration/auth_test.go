//go:build integration

package integration

import (
	"os"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// TestInternalPlain backs up over plaintext with auth-mode INTERNAL, taking the
// password from each place ABS can read it.
func (s *AuthSuite) TestInternalPlain() {
	node := s.startSecuredNode(profilePlain)

	s.Run("literal password", func() {
		s.assertBackupRestoreViaCluster(node, &dto.AerospikeCluster{
			SeedNodes:            []dto.SeedNode{node.Seed},
			UseServicesAlternate: ptr.Of(true),
			Credentials: &dto.Credentials{
				User:     intUser,
				Password: intPassword,
				AuthMode: dto.AuthModeInternal,
			},
		})
	})

	s.Run("password path", func() {
		passwordPath := filepath.Join(s.T().TempDir(), "password.txt")
		s.Require().NoError(os.WriteFile(passwordPath, []byte(intPassword+"\n"), 0o600))

		s.assertBackupRestoreViaCluster(node, &dto.AerospikeCluster{
			SeedNodes:            []dto.SeedNode{node.Seed},
			UseServicesAlternate: ptr.Of(true),
			Credentials: &dto.Credentials{
				User:         intUser,
				PasswordPath: dto.Path(passwordPath),
				AuthMode:     dto.AuthModeInternal,
			},
		})
	})

	s.Run("password from secret agent", func() {
		// password is a secret-agent reference (secrets:<resource>:<key>), not the
		// literal Aerospike password. ABS fetches the real value at connect time.
		agent := s.startSecretAgent(intPassword)

		s.assertBackupRestoreViaCluster(node, &dto.AerospikeCluster{
			SeedNodes:            []dto.SeedNode{node.Seed},
			UseServicesAlternate: ptr.Of(true),
			Credentials: &dto.Credentials{
				User:     intUser,
				Password: redact.Secret(secretRef()),
				AuthMode: dto.AuthModeInternal,
				SecretAgentConfig: dto.SecretAgentConfig{
					SecretAgent: agent,
				},
			},
		})
	})
}

// TestInternalServerTLS backs up with auth-mode INTERNAL over TLS where only the
// server presents a certificate, trusted through ca-path.
func (s *AuthSuite) TestInternalServerTLS() {
	node := s.startSecuredNode(profileServerTLS)

	s.assertBackupRestoreViaCluster(node, &dto.AerospikeCluster{
		SeedNodes:            []dto.SeedNode{node.Seed},
		UseServicesAlternate: ptr.Of(true),
		Credentials: &dto.Credentials{
			User:     intUser,
			Password: intPassword,
			AuthMode: dto.AuthModeInternal,
		},
		// Server authenticates to us; we do not present a client certificate.
		// Trust comes from ca-path. SNI is seed-nodes[].tls-name, not tls.name
		// (name is only valid together with cert-file and key-file).
		TLS: s.serverOnlyTLS(),
	})
}

// TestMutualTLS backs up over mutual TLS with an encrypted client key, both as an
// INTERNAL user and as a PKI user identified by the certificate alone.
func (s *AuthSuite) TestMutualTLS() {
	node := s.startSecuredNode(profileMutualTLS)

	s.Run("internal", func() {
		s.assertBackupRestoreViaCluster(node, &dto.AerospikeCluster{
			SeedNodes:            []dto.SeedNode{node.Seed},
			UseServicesAlternate: ptr.Of(true),
			Credentials: &dto.Credentials{
				User:     intUser,
				Password: intPassword,
				AuthMode: dto.AuthModeInternal,
			},
			TLS: s.mutualTLS(s.certs.Internal),
		})
	})

	s.Run("pki", func() {
		s.assertBackupRestoreViaCluster(node, &dto.AerospikeCluster{
			SeedNodes:            []dto.SeedNode{node.Seed},
			UseServicesAlternate: ptr.Of(true),
			Credentials: &dto.Credentials{
				AuthMode: dto.AuthModePKI,
			},
			TLS: s.mutualTLS(s.certs.PKI),
		})
	})

	s.Run("internal with key-file-password from secret agent", func() {
		// key-file-password is a secret-agent reference here, not the literal
		// passphrase that unlocks the client key. ABS must resolve it through
		// Credentials.SecretAgent before decrypting the key - the same way it
		// resolves Credentials.Password in TestInternalPlain, but for the TLS key
		// file instead of the account password.
		agent := s.startSecretAgent(keyPassword)

		tlsConfig := s.mutualTLS(s.certs.Internal)
		tlsConfig.KeyfilePassword = redact.Secret(secretRef())

		s.assertBackupRestoreViaCluster(node, &dto.AerospikeCluster{
			SeedNodes:            []dto.SeedNode{node.Seed},
			UseServicesAlternate: ptr.Of(true),
			Credentials: &dto.Credentials{
				User:     intUser,
				Password: intPassword,
				AuthMode: dto.AuthModeInternal,
				SecretAgentConfig: dto.SecretAgentConfig{
					SecretAgent: agent,
				},
			},
			TLS: tlsConfig,
		})
	})
}
