//go:build integration

package integration

import (
	"os"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
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
				AuthMode: string(model.AuthModeInternal),
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
}

func (s *AuthSuite) TestInternalServerTLS() {
	cluster := s.startAndProvision(profileServerTLS)

	s.testAuthenticatedBackup(cluster, &dto.AerospikeCluster{
		SeedNodes:            []dto.SeedNode{cluster.seed},
		UseServicesAlternate: ptr.Of(true),
		Credentials: &dto.Credentials{
			User:     intUser,
			Password: intPassword,
			AuthMode: string(model.AuthModeInternal),
		},
		TLS: &dto.TLS{
			ClientTLS: dto.ClientTLS{CAFile: s.certs.caCert},
		},
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
				AuthMode: string(model.AuthModeInternal),
			},
			TLS: &dto.TLS{
				ClientTLS: dto.ClientTLS{
					CAFile:   s.certs.caCert,
					Name:     tlsName,
					Certfile: s.certs.internalCert,
					Keyfile:  s.certs.internalKey,
				},
			},
		})
	})

	s.Run("pki", func() {
		s.testAuthenticatedBackup(cluster, &dto.AerospikeCluster{
			SeedNodes:            []dto.SeedNode{cluster.seed},
			UseServicesAlternate: ptr.Of(true),
			Credentials: &dto.Credentials{
				AuthMode: string(model.AuthModePKI),
			},
			TLS: &dto.TLS{
				ClientTLS: dto.ClientTLS{
					CAFile:   s.certs.caCert,
					Name:     tlsName,
					Certfile: s.certs.pkiCert,
					Keyfile:  s.certs.pkiKey,
				},
			},
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
