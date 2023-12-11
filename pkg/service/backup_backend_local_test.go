package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFolderSize(t *testing.T) {
	// Create a temporary test directory with some files
	var tests = []func() (string, int){noFiles, threeFiles, oneFile, manyLevels}

	for _, test := range tests {
		runTest(t, test)
	}
}

func runTest(t *testing.T, test func() (string, int)) {
	tempDir, expected := test()
	defer cleanupTestDir(tempDir)

	// Test the folderSize function
	size := *folderSize(tempDir)

	// Check if the calculated size matches the expected size
	if size != int64(expected) {
		t.Errorf("Expected size: %d, got: %d", expected, size)
	}
}

func noFiles() (string, int) {
	tempDir := "test_folder"
	os.Mkdir(tempDir, os.ModePerm)
	return tempDir, 0
}

func oneFile() (string, int) {
	tempDir := "test_folder"
	os.Mkdir(tempDir, os.ModePerm)

	createTestFile(tempDir, "file1.txt", 42)

	return tempDir, 42
}

func threeFiles() (string, int) {
	tempDir := "test_folder"
	os.Mkdir(tempDir, os.ModePerm)

	// Create some test files with known sizes
	createTestFile(tempDir, "file1.txt", 100)
	createTestFile(tempDir, "file2.txt", 200)
	os.Mkdir("test_folder/subfolder", os.ModePerm)
	createTestFile(tempDir, "subfolder/file3.txt", 150)

	return tempDir, 450
}

func manyLevels() (string, int) {
	tempDir := "test_folder"
	os.Mkdir(tempDir, os.ModePerm)

	// Create some test files with known sizes
	createTestFile(tempDir, "file.txt", 10)
	os.Mkdir("test_folder/subfolder", os.ModePerm)
	createTestFile(tempDir, "subfolder/file.txt", 10)
	os.Mkdir("test_folder/subfolder/subfolder", os.ModePerm)
	createTestFile(tempDir, "subfolder/subfolder/file.txt", 10)
	os.Mkdir("test_folder/subfolder/subfolder/subfolder", os.ModePerm)
	createTestFile(tempDir, "subfolder/subfolder/subfolder/file.txt", 10)

	return tempDir, 40
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
