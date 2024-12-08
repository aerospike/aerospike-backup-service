package model

import "fmt"

type GcpStorage struct {
	// KeyFile is the path to the JSON file containing the Google Cloud service account key.
	// This file is used for authentication with GCP services.
	KeyFile string
	// BucketName is the name of the GCP bucket where backups will be stored.
	BucketName string
	// Path is the root directory within the GCS bucket where backups will be stored.
	Path string
	// Endpoint is an alternative URL for the GCS API.
	// This should only be used for testing or in specific non-production scenarios.
	Endpoint string
	// Optional secret agent configuration to fetch keyfile from a secret store.
	SecretAgent *SecretAgent
}

func (s *GcpStorage) storage() {}
func (s *GcpStorage) String() string {
	return fmt.Sprintf("GcpStorage(Bucket: %s, Path: %s)", s.BucketName, s.Path)
}
