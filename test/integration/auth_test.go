//go:build integration

package integration

import (
	"os"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	as "github.com/aerospike/aerospike-client-go/v8"
)

func (s *AuthSuite) TestInternalPlain() {
	cluster := s.startAndProvision(profilePlain)

	s.Run("literal password", func() {
		s.testAuthenticatedBackup(cluster, &dto.AerospikeCluster{
			SeedNodes:            []dto.SeedNode{cluster.seed},
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

		s.testAuthenticatedBackup(cluster, &dto.AerospikeCluster{
			SeedNodes:            []dto.SeedNode{cluster.seed},
			UseServicesAlternate: ptr.Of(true),
			Credentials: &dto.Credentials{
				User:         intUser,
				PasswordPath: passwordPath,
				AuthMode:     string(model.AuthModeInternal),
			},
		})
	})

	s.Run("password from secret agent", func() {
		// password is a secret-agent reference (secrets:<resource>:<key>), not the
		// literal Aerospike password. ABS fetches the real value at connect time.
		agent := s.startSecretAgent(intPassword)

		s.testAuthenticatedBackup(cluster, &dto.AerospikeCluster{
			SeedNodes:            []dto.SeedNode{cluster.seed},
			UseServicesAlternate: ptr.Of(true),
			Credentials: &dto.Credentials{
				User:     intUser,
				Password: decoder.Secret(secretRef()),
				AuthMode: string(model.AuthModeInternal),
				SecretAgentConfig: dto.SecretAgentConfig{
					SecretAgent: agent,
				},
			},
		})
	})
}

func (s *AuthSuite) TestInternalServerTLS() {
	cluster := s.startAndProvision(profileServerTLS)

	s.testAuthenticatedBackup(cluster, &dto.AerospikeCluster{
		SeedNodes:            []dto.SeedNode{cluster.seed},
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

func (s *AuthSuite) TestMutualTLS() {
	cluster := s.startAndProvision(profileMutualTLS)

	s.Run("internal", func() {
		s.testAuthenticatedBackup(cluster, &dto.AerospikeCluster{
			SeedNodes:            []dto.SeedNode{cluster.seed},
			UseServicesAlternate: ptr.Of(true),
			Credentials: &dto.Credentials{
				User:     intUser,
				Password: intPassword,
				AuthMode: dto.AuthModeInternal,
			},
			TLS: s.mutualTLS(s.certs.internalCert, s.certs.internalKeyEncrypted),
		})
	})

	s.Run("pki", func() {
		s.testAuthenticatedBackup(cluster, &dto.AerospikeCluster{
			SeedNodes:            []dto.SeedNode{cluster.seed},
			UseServicesAlternate: ptr.Of(true),
			Credentials: &dto.Credentials{
				AuthMode: dto.AuthModePKI,
			},
			TLS: s.mutualTLS(s.certs.pkiCert, s.certs.pkiKeyEncrypted),
		})
	})
}

func (s *AuthSuite) testAuthenticatedBackup(cluster authCluster, config *dto.AerospikeCluster) {
	s.Require().NoError(cluster.adminClient.Truncate(nil, namespace, "", nil))
	s.seedAuthRecords(cluster.adminClient, []int{10, 20, 30})

	e := s.setupEnv(func(c *dto.Config) {
		c.AerospikeClusters[clusterName] = config
	})
	s.triggerFullBackup(e)
	backup := s.waitForFullBackup(e)

	s.assertBackupDetails(backup, 3)
}

func (s *AuthSuite) seedAuthRecords(client *as.Client, ages []int) {
	writePolicy := as.NewWritePolicy(0, 0)
	for i, age := range ages {
		key, err := as.NewKey(namespace, setName, i)
		s.Require().NoError(err)
		s.Require().NoError(client.Put(writePolicy, key, as.BinMap{"age": age}))
	}
}

// dto.TLS is ABS's client-side TLS config for Aerospike. It does not configure the
// database; it tells this process how to verify the server and, for mTLS, how to
// prove who we are.
//
// Handshake, in one pass:
//  1. We open a TLS connection and send SNI (the name we expect the cert to use).
//  2. The server presents its certificate. We trust it only if it chains to a CA
//     we loaded from ca-file or ca-path.
//  3. If the server asks for a client cert (tls-authenticate-client), we present
//     cert-file, proving possession with key-file (unlocked by key-file-password
//     when the PEM is encrypted).
//  4. Both sides agree a protocol version (protocols) and a cipher (cipher-suite).
//
// ca-file and ca-path are mutually exclusive. These helpers split them:
// server-only TLS uses ca-path; mTLS uses ca-file. Together they set every field.
func (s *AuthSuite) serverOnlyTLS() *dto.TLS {
	return &dto.TLS{
		ClientTLS: dto.ClientTLS{
			// ca-file is the other way to load CAs (a single PEM bundle). Left empty
			// here because ca-path is set below; both at once is a validation error.
			CAFile: "",
			// name is SNI / ServerName on the cluster TLS object. Validation requires
			// it together with cert-file and key-file, so server-only TLS cannot set
			// it. The seed node's tls-name (already set on cluster.seed) is what the
			// Aerospike client sends as SNI in this profile.
			Name:     "",
			Certfile: "",
			Keyfile:  "",
		},
		// ca-path is a directory of PEM CA files. Same purpose as ca-file: decide
		// whether the server's certificate is trusted. Used when CAs are dropped
		// in as separate files (for example a Kubernetes secret volume).
		CAPath: s.certs.caDir,
		// protocols is Apache SSLProtocol syntax, space-separated. ABS currently
		// accepts TLSv1.2 only. One token pins both the minimum and the maximum.
		Protocols: tlsProtocols,
		// cipher-suite is colon-separated IANA names (not OpenSSL nicknames). It
		// restricts which TLS 1.2 ciphers we offer. Empty means Go's defaults.
		CipherSuite: tlsCipherSuite,
		// key-file-password is only meaningful with key-file (encrypted client key).
		KeyfilePassword: "",
	}
}

func (s *AuthSuite) mutualTLS(certFile, encryptedKeyFile string) *dto.TLS {
	return &dto.TLS{
		ClientTLS: dto.ClientTLS{
			// ca-file: PEM bundle of CAs we trust to sign the server certificate.
			CAFile: s.certs.caCert,
			// name: hostname we put in SNI and then check against the server cert.
			// Must be set together with cert-file and key-file.
			Name: tlsName,
			// cert-file: our client certificate. The server uses this to decide
			// who we are. For auth-mode PKI the Aerospike username is the cert CN.
			Certfile: certFile,
			// key-file: private key matching cert-file. Never sent on the wire;
			// used to prove we own the certificate. Encrypted in these tests.
			Keyfile: encryptedKeyFile,
		},
		// ca-path left empty: mutually exclusive with ca-file.
		CAPath: "",
		// protocols / cipher-suite: same meaning as in serverOnlyTLS.
		Protocols:   tlsProtocols,
		CipherSuite: tlsCipherSuite,
		// key-file-password: passphrase for an encrypted key-file PEM. Can also be
		// a secrets:<resource>:<key> reference when a secret agent is configured.
		KeyfilePassword: clientKeyPassword,
	}
}
