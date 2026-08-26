package main

import "os"

var (
	readFile  = os.ReadFile
	writeFile = func(path string, data []byte) error {
		return os.WriteFile(path, data, 0600)
	}
	removeFile = os.Remove
)
