package secrets

import (
	"os"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testdataFolder = "./testdata"
	passwordPath   = testdataFolder + "/password.txt"
)

func TestResolve(t *testing.T) {
	resolver := NewPasswordResolver(NewResolver())

	tests := []struct {
		name             string
		setupMock        func()
		credentials      *model.Credentials
		expectedPassword *string
		expectedErr      bool
	}{
		{
			name:      "ValidPasswordPath",
			setupMock: func() { createFile("password") },
			credentials: &model.Credentials{
				PasswordPath: passwordPath,
			},
			expectedPassword: ptr.Of("password"),
			expectedErr:      false,
		},
		{
			name:      "ValidPasswordPathWithLF",
			setupMock: func() { createFile("password\n") },
			credentials: &model.Credentials{
				PasswordPath: passwordPath,
			},
			expectedPassword: ptr.Of("password"),
			expectedErr:      false,
		},
		{
			name:      "ValidPasswordPathWithCRLF",
			setupMock: func() { createFile("password\r\n") },
			credentials: &model.Credentials{
				PasswordPath: passwordPath,
			},
			expectedPassword: ptr.Of("password"),
			expectedErr:      false,
		},
		{
			name:      "InvalidPasswordPath",
			setupMock: func() {},
			credentials: &model.Credentials{
				PasswordPath: "not-existing.txt",
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
				Password: "plain",
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

			password, err := resolver.Resolve(t.Context(), test.credentials)
			if test.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedPassword, password)
			}
			_ = os.RemoveAll(testdataFolder)
		})
	}
}

func createFile(content string) {
	text := []byte(content)
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
