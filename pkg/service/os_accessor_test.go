package service

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestDeleteFolder(t *testing.T) {
	parentFolder := tempFolder + "/parent"
	folderToDelete := parentFolder + "/nested"
	_ = os.MkdirAll(folderToDelete, 0744)
	_ = os.WriteFile(folderToDelete+"/file.txt", []byte("hello world"), 0600)

	err := NewOSDiskAccessor().DeleteFolder(folderToDelete)

	if err != nil {
		t.Fatalf("Unexpected error deleting directory: %v", err)
	}
	_, err = os.Stat(folderToDelete)
	if !os.IsNotExist(err) {
		t.Fatalf("Nested folder %s was not deleted", folderToDelete)
	}
	_, err = os.Stat(parentFolder)
	if !os.IsNotExist(err) {
		t.Fatalf("Parent folder %s was not deleted", parentFolder)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempFolder)
	})
}

func TestLsDir(t *testing.T) {
	testCases := []struct {
		name     string
		setup    func() string
		after    *string
		expected []string
	}{
		{
			name: "Existing directory without filter",
			setup: func() string {
				dir := t.TempDir()
				_ = os.MkdirAll(filepath.Join(dir, "subDir1"), 0755)
				_ = os.MkdirAll(filepath.Join(dir, "subDir2"), 0755)
				_ = os.MkdirAll(filepath.Join(dir, "subDir3"), 0755)
				return dir
			},
			after:    nil,
			expected: []string{"subDir1", "subDir2", "subDir3"},
		},
		{
			name: "Existing directory with filter",
			setup: func() string {
				dir := t.TempDir()
				_ = os.MkdirAll(filepath.Join(dir, "subDir1"), 0755)
				_ = os.MkdirAll(filepath.Join(dir, "subDir2"), 0755)
				_ = os.MkdirAll(filepath.Join(dir, "subDir3"), 0755)
				return dir
			},
			after:    util.Ptr("subDir2"),
			expected: []string{"subDir2", "subDir3"},
		},
		{
			name: "Non existing directory",
			setup: func() string {
				return filepath.Join(t.TempDir(), "non-existing-dir")
			},
			after:    nil,
			expected: []string{},
		},
		{
			name: "File instead of directory",
			setup: func() string {
				dir := t.TempDir()
				_ = os.WriteFile(filepath.Join(dir, "file"), []byte("test content"), 0600)
				return dir
			},
			after:    nil,
			expected: []string{},
		},
		{
			name: "Mixed content with filter",
			setup: func() string {
				dir := t.TempDir()
				_ = os.MkdirAll(filepath.Join(dir, "aDir"), 0755)
				_ = os.MkdirAll(filepath.Join(dir, "bDir"), 0755)
				_ = os.WriteFile(filepath.Join(dir, "cFile"), []byte("test content"), 0600)
				_ = os.MkdirAll(filepath.Join(dir, "dDir"), 0755)
				return dir
			},
			after:    util.Ptr("bDir"),
			expected: []string{"bDir", "dDir"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := tc.setup()
			result, err := NewOSDiskAccessor().lsDir(dir, tc.after)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result) != len(tc.expected) {
				t.Fatalf("Unexpected number of results\nExpected: %v\nGot: %v", tc.expected, result)
			}

			for i, exp := range tc.expected {
				if !strings.HasSuffix(result[i], exp) {
					t.Errorf("Unexpected result at index %d\nExpected suffix: %s\nGot: %s", i, exp, result[i])
				}
			}
		})
	}
}

func TestLsFiles(t *testing.T) {
	testCases := []struct {
		name     string
		setup    func() string
		expected []string
	}{
		{
			name: "Existing directory",
			setup: func() string {
				dir := t.TempDir()
				subDir1 := filepath.Join(dir, "subDir1")
				subDir2 := filepath.Join(dir, "subDir2")
				_ = os.MkdirAll(subDir1, 0755)
				_ = os.MkdirAll(subDir2, 0755)
				return dir
			},
			expected: nil,
		},
		{
			name: "Non existing directory",
			setup: func() string {
				dir := filepath.Join(t.TempDir(), "non-existing-dir")
				return dir
			},
			expected: []string{},
		},
		{
			name: "File instead of directory",
			setup: func() string {
				dir := t.TempDir()
				file := filepath.Join(dir, "file")
				_ = os.WriteFile(file, []byte("test content"), 0600)
				return dir
			},
			expected: []string{"file"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := tc.setup()
			result, err := NewOSDiskAccessor().lsFiles(dir)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result) != len(tc.expected) {
				t.Fatalf("Unexpected results \nExpected: %v \nGot: %v", tc.expected, result)
			}
		})
	}
}

func TestValidatePathContainsBackup(t *testing.T) {
	dir := t.TempDir()

	// Prepare some test cases
	testCases := []struct {
		name string
		path string
		file bool
		err  error
	}{
		{
			name: "PathDoesNotExist",
			path: "/invalid/path",
			file: false,
			err: &os.PathError{
				Op:   "stat",
				Path: "/invalid/path",
				Err:  syscall.ENOENT,
			},
		},
		{
			name: "PathExistsButNoBackupFile",
			path: dir,
			file: false,
			err:  fmt.Errorf("no backup files found in %s", dir),
		},
		{
			name: "PathExistsWithBackupFile",
			path: dir,
			file: true,
			err:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// If necessary, create a backup file in the test directory
			if tc.file {
				_, err := os.Create(dir + "/testfile.asb")
				if err != nil {
					t.Fatalf("Failed to create test file: %s", err)
				}
			}

			// Run the function and check its output
			_, err := validatePathContainsBackup(tc.path)
			if reflect.TypeOf(err) != reflect.TypeOf(tc.err) {
				t.Errorf("Expected error %v, but got %v", tc.err, err)
			}
		})
	}
}

func TestReadBackupDetails(t *testing.T) {
	accessor := NewOSDiskAccessor()

	path := filepath.Join(os.TempDir(), "test.yaml")
	_ = os.MkdirAll(path, fs.ModePerm)
	metadata := dto.BackupMetadata{
		Created:   time.Now(),
		Namespace: "test-backup",
	}

	data, _ := json.Marshal(metadata)
	_ = accessor.write(filepath.Join(path, metadataFile), data)

	details, err := accessor.readBackupDetails(path, true)
	assert.NoError(t, err)
	assert.True(t, metadata.Created.Equal(details.BackupMetadata.Created))
	assert.Equal(t, path, *details.Key)
}

func TestReadBackupDetailsNegative(t *testing.T) {
	accessor := &OSDiskAccessor{}
	tests := []struct {
		name  string
		setup func() string
	}{
		{
			name: "NonExistentDir",
			setup: func() string {
				return "nonexistentdir"
			},
		},
		{
			name: "EmptyDir",
			setup: func() string {
				return t.TempDir()
			},
		},
		{
			name: "InvalidMetadata",
			setup: func() string {
				dir := t.TempDir()
				_ = accessor.write(filepath.Join(dir, metadataFile), []byte{1, 2, 3})
				return dir
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup()
			_, err := accessor.readBackupDetails(dir, false)
			assert.Error(t, err)
		})
	}
}

func TestReadState(t *testing.T) {
	accessor := NewOSDiskAccessor()

	dir := os.TempDir()
	_ = os.MkdirAll(dir, fs.ModePerm)
	path := filepath.Join(dir, "test_state.yaml")
	expected := &dto.BackupState{
		Performed: 10,
	}
	data, _ := json.Marshal(expected)
	_ = accessor.write(path, data)

	state := &dto.BackupState{}
	err := accessor.readBackupState(path, state)
	assert.NoError(t, err)
	assert.Equal(t, expected, state)
}

func TestReadStateNegative(t *testing.T) {
	accessor := &OSDiskAccessor{}
	tests := []struct {
		name      string
		setup     func() string
		ignoreErr bool
	}{
		{
			name: "NonExistentDir",
			setup: func() string {
				return "nonexistentdir"
			},
			ignoreErr: true, // when state file not exists, default is returned.
		},
		{
			name: "EmptyDir",
			setup: func() string {
				return t.TempDir()
			},
		},
		{
			name: "InvalidState",
			setup: func() string {
				dir := t.TempDir()
				path := filepath.Join(dir, "test_state.yaml")
				_ = accessor.write(path, []byte{1, 2, 3})
				return path
			},
			ignoreErr: true, // when state file corrupted, default is returned.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup()
			state := &dto.BackupState{}
			err := accessor.readBackupState(dir, state)
			if tt.ignoreErr {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
