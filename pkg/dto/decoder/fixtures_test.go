package decoder

import "time"

const (
	literalPassword    = "superSecretPassword"
	literalAccessKey   = "test-access-key-id"
	literalSecretKey   = "test-secret-access-key"
	literalTLSPassword = "tls-key-password"
	validSecretRef     = "secrets:resource:key"
	malformedSecretRef = "secrets:foo"
)

type testCredentials struct {
	User     string `json:"user" yaml:"user"`
	Password Secret `json:"password" yaml:"password"`
}

type testTLS struct {
	KeyfilePassword Secret `json:"keyfile-password" yaml:"keyfile-password"`
}

type testEncryption struct {
	KeySecret Secret `json:"key-secret" yaml:"key-secret"`
}

type testStorage struct {
	Name            string `json:"name" yaml:"name"`
	AccessKeyID     Secret `json:"access-key-id" yaml:"access-key-id"`
	SecretAccessKey Secret `json:"secret-access-key" yaml:"secret-access-key"`
}

type testBackupRoutine struct {
	Name     string        `json:"name" yaml:"name"`
	Storages []testStorage `json:"storages" yaml:"storages"`
	Keys     [2]Secret     `json:"keys" yaml:"keys"`
}

type testCluster struct {
	Credentials *testCredentials `json:"credentials" yaml:"credentials"`
	TLS         *testTLS         `json:"tls" yaml:"tls"`
	Encryption  testEncryption   `json:"encryption" yaml:"encryption"`
}

type testConfig struct {
	AerospikeClusters map[string]*testCluster        `json:"aerospike-clusters" yaml:"aerospike-clusters"`
	StorageProviders  map[string]testStorage         `json:"storage-providers" yaml:"storage-providers"`
	Routines          []testBackupRoutine            `json:"routines" yaml:"routines"`
	BackupHistory     map[string][]testBackupDetails `json:"backup-history" yaml:"backup-history"`
}

type testBackupDetails struct {
	Key       string    `json:"key" yaml:"key"`
	Created   time.Time `json:"created" yaml:"created"`
	Timestamp int64     `json:"timestamp" yaml:"timestamp"`
	Finished  time.Time `json:"finished" yaml:"finished"`
}

var testCreds = &testCredentials{
	User:     "testUser",
	Password: literalPassword,
}

func testConfigWithCreds(creds *testCredentials) testConfig {
	return testConfig{
		AerospikeClusters: map[string]*testCluster{
			"cluster1": {Credentials: creds},
		},
	}
}

func testComplexConfig() testConfig {
	created := time.UnixMilli(1000).UTC()
	finished := time.UnixMilli(5000).UTC()

	return testConfig{
		AerospikeClusters: map[string]*testCluster{
			"cluster1": {
				Credentials: &testCredentials{
					User:     "testUser",
					Password: literalPassword,
				},
				TLS: &testTLS{KeyfilePassword: literalTLSPassword},
				Encryption: testEncryption{
					KeySecret: literalSecretKey,
				},
			},
			"cluster2": {
				Credentials: &testCredentials{
					User:     "refUser",
					Password: validSecretRef,
				},
				TLS: nil,
				Encryption: testEncryption{
					KeySecret: malformedSecretRef,
				},
			},
			"cluster3": {
				Credentials: &testCredentials{
					User: "emptyPasswordUser",
				},
			},
		},
		StorageProviders: map[string]testStorage{
			"s3-main": {
				Name:            "main-bucket",
				AccessKeyID:     literalAccessKey,
				SecretAccessKey: literalSecretKey,
			},
			"s3-ref": {
				Name:            "ref-bucket",
				AccessKeyID:     validSecretRef,
				SecretAccessKey: Secret(""),
			},
		},
		Routines: []testBackupRoutine{
			{
				Name: "daily",
				Storages: []testStorage{
					{
						Name:            "routine-storage",
						AccessKeyID:     literalAccessKey,
						SecretAccessKey: literalSecretKey,
					},
				},
				Keys: [2]Secret{literalPassword, validSecretRef},
			},
		},
		BackupHistory: map[string][]testBackupDetails{
			"routine1": {
				{
					Key:       "backup1",
					Created:   created,
					Timestamp: 1000,
					Finished:  finished,
				},
			},
		},
	}
}

func literalSecretsInConfig() []string {
	return []string{
		literalPassword,
		literalAccessKey,
		literalSecretKey,
		literalTLSPassword,
	}
}

func nonSecretValuesInConfig() []string {
	return []string{
		"testUser",
		"refUser",
		"emptyPasswordUser",
		"main-bucket",
		"ref-bucket",
		"routine-storage",
		"daily",
		"backup1",
		validSecretRef,
	}
}
