package model

import "fmt"

type GcpStorage struct {
	// KeyFile is the path to the JSON file containing the Google Cloud service account key.
	// This file is used for authentication with GCP services.
	KeyFile string
	// KeyJSON is the contents of the Google Cloud service account key.
	KeyJSON string
	// BucketName is the name of the GCP bucket where backups will be stored.
	BucketName string
	// Path is the root directory within the GCS bucket where backups will be stored.
	Path string
	// Endpoint is an alternative URL for the GCS API.
	// This should only be used for testing or in specific non-production scenarios.
	Endpoint string
	// SecretAgent configuration to fetch keyfile from a secret store (optional).
	SecretAgent *SecretAgent
}

func (s *GcpStorage) GetStorageClass() StorageClass {
	return StorageClass{}
}

func (s *GcpStorage) GetPath() string {
	return s.Path
}

func (s *GcpStorage) String() string {
	return fmt.Sprintf("GcpStorage(Bucket: %s, Path: %s)", s.BucketName, s.Path)
}
