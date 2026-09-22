package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/stretchr/testify/assert"
)

func routineBody(e validTestBackupEntities) string {
	return marshalToString(dto.BackupRoutine{
		SourceCluster: e.clusterName,
		Storage:       e.storageName,
		BackupPolicy:  e.policyName,
		IntervalCron:  "@daily",
		Namespaces:    &[]dto.NamespaceName{"source-ns1"},
	})
}

func storageBody(validTestBackupEntities) string {
	return marshalToString(dto.Storage{LocalStorage: &dto.LocalStorage{Path: "/tmp"}})
}

func clusterBody(validTestBackupEntities) string {
	return marshalToString(cluster)
}

func policyBody(validTestBackupEntities) string {
	return "{}"
}

// A config-mutation endpoint reports a name it cannot resolve the way the matching GET does, and a
// clash with the stored configuration as a conflict, so a client can tell "you sent nonsense" from
// "that entity is not there" and "that entity is already there" by status code alone.
func TestConfigChangeStatusCodes(t *testing.T) {
	tests := []struct {
		name           string
		handler        func(*Service) http.HandlerFunc
		entityName     string
		body           func(validTestBackupEntities) string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "add a routine that already exists",
			handler:        func(s *Service) http.HandlerFunc { return s.AddRoutine },
			entityName:     "routine1",
			body:           routineBody,
			expectedStatus: http.StatusConflict,
			expectedError:  `routine "routine1" already exists`,
		},
		{
			name:           "add a storage that already exists",
			handler:        func(s *Service) http.HandlerFunc { return s.AddStorage },
			entityName:     "storage1",
			body:           storageBody,
			expectedStatus: http.StatusConflict,
			expectedError:  `storage "storage1" already exists`,
		},
		{
			name:           "add a cluster that already exists",
			handler:        func(s *Service) http.HandlerFunc { return s.AddAerospikeCluster },
			entityName:     "cluster1",
			body:           clusterBody,
			expectedStatus: http.StatusConflict,
			expectedError:  `cluster "cluster1" already exists`,
		},
		{
			name:           "add a policy that already exists",
			handler:        func(s *Service) http.HandlerFunc { return s.AddPolicy },
			entityName:     "test-policy",
			body:           policyBody,
			expectedStatus: http.StatusConflict,
			expectedError:  `policy "test-policy" already exists`,
		},
		{
			name:           "update a routine that does not exist",
			handler:        func(s *Service) http.HandlerFunc { return s.UpdateRoutine },
			entityName:     "no-such-routine",
			body:           routineBody,
			expectedStatus: http.StatusNotFound,
			expectedError:  `routine "no-such-routine" not found`,
		},
		{
			name:           "update a storage that does not exist",
			handler:        func(s *Service) http.HandlerFunc { return s.UpdateStorage },
			entityName:     "no-such-storage",
			body:           storageBody,
			expectedStatus: http.StatusNotFound,
			expectedError:  `storage "no-such-storage" not found`,
		},
		{
			name:           "update a cluster that does not exist",
			handler:        func(s *Service) http.HandlerFunc { return s.UpdateAerospikeCluster },
			entityName:     "no-such-cluster",
			body:           clusterBody,
			expectedStatus: http.StatusNotFound,
			expectedError:  `cluster "no-such-cluster" not found`,
		},
		{
			name:           "update a policy that does not exist",
			handler:        func(s *Service) http.HandlerFunc { return s.UpdatePolicy },
			entityName:     "no-such-policy",
			body:           policyBody,
			expectedStatus: http.StatusNotFound,
			expectedError:  `policy "no-such-policy" not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newServiceWithNamespaceValidator(t)
			entities := addValidBackupConfig(svc)

			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodPost, "/v1/config/"+tt.entityName, strings.NewReader(tt.body(entities)))
			req.SetPathValue("name", tt.entityName)
			w := httptest.NewRecorder()

			tt.handler(svc)(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), tt.expectedError)
		})
	}
}
