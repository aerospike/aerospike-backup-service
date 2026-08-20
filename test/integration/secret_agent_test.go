//go:build integration

package integration

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
)

// TestBackupRestoreWithSecretAgentEncryption starts Aerospike Secret Agent with the
// local file backend, fetches the backup encryption key from it, then restores
// with the same key.
func (s *Suite) TestBackupRestoreWithSecretAgentEncryption() {
	pemKey, err := generateEncryptionPEM()
	s.Require().NoError(err)

	agent := s.startSecretAgent(pemKey)

	e := s.setupEnv(func(c *dto.Config) {
		c.SecretAgents = map[string]*dto.SecretAgent{
			secretAgentName: agent,
		}
		c.BackupPolicies[policyName].EncryptionPolicy = &dto.EncryptionPolicy{
			Mode:      dto.EncryptAES128,
			KeySecret: decoder.Secret(secretRef()),
		}
		c.BackupRoutines[routineName].SecretAgent = secretAgentName
	})

	s.seedRecords([]int{10, 20, 30})

	s.triggerFullBackup(e)
	fullBackup := s.waitForFullBackup(e)
	s.assertBackupDetails(fullBackup, 3)

	s.Require().NoError(s.client.Truncate(nil, namespace, "", nil))

	req := defaultRestoreRequest(fullBackup.Key)
	req.SecretAgentConfig = &dto.SecretAgentConfig{
		SecretAgentName: secretAgentName,
	}
	req.Policy = &dto.RestorePolicy{
		BaseRestorePolicy: dto.BaseRestorePolicy{
			EncryptionPolicy: &dto.EncryptionPolicy{
				Mode:      dto.EncryptAES128,
				KeySecret: decoder.Secret(secretRef()),
			},
		},
	}

	status := s.restoreByPath(e, req)

	s.Equal(dto.RestoreSuccess, status.Status)
	s.Equal(uint64(3), status.InsertedRecords)
	s.Empty(status.Error)

	s.assertRecordsRestored([]int{10, 20, 30})
}
