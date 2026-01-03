package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogger(t *testing.T) {
	// Create temp dir for logs
	tmpDir, err := os.MkdirTemp("", "perssh-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Since NewLogger/InitLogger uses os.Executable() path, we might need to be careful.
	// However, our implementation uses it to find where the binary is.
	// For testing purposes, we can test the Audit and System methods if we can inject the files.
	// But our current InitLogger is tied to os.Executable.
	
	// Let's at least test EnsureDir which is a utility
	testDir := filepath.Join(tmpDir, "subdir")
	err = EnsureDir(testDir)
	if err != nil {
		t.Errorf("EnsureDir failed: %v", err)
	}
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Errorf("EnsureDir did not create the directory")
	}
}
