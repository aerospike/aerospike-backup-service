package secrets

import (
	"os"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
)

const (
	testdataFolder = "./testdata"
	passwordPath   = testdataFolder + "/password.txt"
)

func TestResolve(t *testing.T) {
	resolver := NewPasswordResolver(NewResolver(t.Context()))

	tests := []struct {
		name             string
		setupMock        func()
		credentials      *model.Credentials
		expectedPassword *string
		expectedErr      bool
	}{
		{
			name:      "ValidPasswordPath",
			setupMock: createValidFile,
			credentials: &model.Credentials{
				User:         nil,
				PasswordPath: ptr.Of(passwordPath),
			},
			expectedPassword: ptr.Of("password"),
			expectedErr:      false,
		},
		{
			name:      "InvalidPasswordPath",
			setupMock: func() {},
			credentials: &model.Credentials{
				User:         nil,
				PasswordPath: ptr.Of("not-existing.txt"),
			},
			expectedPassword: nil,
			expectedErr:      true,
		},
		{
			name:             "NoCredentials",
			setupMock:        func() {},
			expectedPassword: nil,
			expectedErr:      false, // Returns nil, nil
		},
		{
			name:             "NoPassword",
			setupMock:        func() {},
			credentials:      &model.Credentials{},
			expectedPassword: nil,
			expectedErr:      true,
		},
		{
			name:      "PlainPassword",
			setupMock: func() {},
			credentials: &model.Credentials{
				Password: ptr.Of("plain"),
			},
			expectedPassword: ptr.Of("plain"),
			expectedErr:      false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.setupMock != nil {
				test.setupMock()
			}

			password, err := resolver.Resolve(test.credentials)
			if test.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedPassword, password)
			}
			_ = os.RemoveAll(testdataFolder)
		})
	}
}

// Note: Logic for checking file caching was removed because Resolver is stateless regarding files.
// Logic for Secret Agent caching is tested in secret_agent_test.go (if we create it).

func createValidFile() {
	text := []byte("password")
	_ = os.MkdirAll(testdataFolder, 0744)
	f, _ := os.OpenFile(passwordPath, os.O_WRONLY|os.O_CREATE, 0644)
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			panic(err)
		}
	}(f)
	_, _ = f.Write(text)
}
