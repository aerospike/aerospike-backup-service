package main

import (
	"github.com/aerospike/backup/pkg/service"
)

func backendsToReaders(backends map[string]service.BackupBackend) map[string]service.BackupListReader {
	result := make(map[string]service.BackupListReader)
	for key, value := range backends {
		result[key] = value
	}
	return result
}
