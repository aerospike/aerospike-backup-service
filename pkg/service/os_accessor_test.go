package service

import (
	"os"
	"testing"
)

func TestDeleteFolder(t *testing.T) {
	diskAccessor := OSDiskAccessor{}
	// Create a temporary directory
	parentFolder := tempFolder + "/parent"
	folderToDelete := parentFolder + "/nested"
	_ = os.MkdirAll(folderToDelete, 0744)

	// Create a file within the temporary directory
	_ = os.WriteFile(folderToDelete+"/file.txt", []byte("hello world"), 0666)

	err := diskAccessor.DeleteFolder(folderToDelete)
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
