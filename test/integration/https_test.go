//go:build integration

package integration

// TestHTTPSListenerWithSecretAgentKeyPassword serves the ABS API over HTTPS only, from
// a private key encrypted on disk whose passphrase ABS fetches from Secret Agent when
// it builds the listener, and runs a full backup through it.
func (s *BackupSuite) TestHTTPSListenerWithSecretAgentKeyPassword() {
	certs := s.generateHTTPSCertificates()
	agent := s.startSecretAgent(keyPassword)
	e := s.setupHTTPSEnv(certs, agent)

	s.seedRecords([]int{10, 20, 30})
	s.triggerFullBackup(e)

	s.assertBackupDetails(s.waitForFullBackup(e), 3)
}
