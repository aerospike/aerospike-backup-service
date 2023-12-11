package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFolderSize(t *testing.T) {
	// Create a temporary test directory with some files
	tempDir := setupTestDir()
	defer cleanupTestDir(tempDir)

	// Test the folderSize function
	size := *folderSize(tempDir)

	// Check if the calculated size matches the expected size
	expected := 450
	if size != int64(expected) {
		t.Errorf("Expected size: %d, got: %d", expected, size)
	}
}

func setupTestDir() string {
	tempDir := "test_folder"
	err := os.Mkdir(tempDir, os.ModePerm)
	if err != nil {
		panic("Error creating test directory: " + err.Error())
	}

	// Create some test files with known sizes
	createTestFile(tempDir, "file1.txt", 100)
	createTestFile(tempDir, "file2.txt", 200)
	os.Mkdir("test_folder/subfolder", os.ModePerm)
	createTestFile(tempDir, "subfolder/file3.txt", 150)

	return tempDir
}

func createTestFile(dir, name string, size int) {
	filePath := filepath.Join(dir, name)
	file, _ := os.Create(filePath)
	defer file.Close()

	content := make([]byte, size)
	file.Write(content)
}

func cleanupTestDir(dir string) {
	err := os.RemoveAll(dir)
	if err != nil {
		panic("Error cleaning up test directory: " + err.Error())
	}
}
