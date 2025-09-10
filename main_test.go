package main

import (
	"os"
	"testing"
)

func TestNewServer(t *testing.T) {
	t.Run("should return a new server when the config is valid", func(t *testing.T) {
		cfg := &Config{
			DatabaseURL: ":memory:",
			Port:        "7000",
		}
		s := NewServer(cfg)
		if s == nil {
			t.Error("expected server to not be nil")
		}
	})

	t.Run("should return nil when the config is nil", func(t *testing.T) {
		s := NewServer(nil)
		if s != nil {
			t.Error("expected server to be nil")
		}
	})

	t.Run("should return nil when the database url is invalid", func(t *testing.T) {
		// Create a temporary directory for the test
		dir := t.TempDir()

		// Create a file in the directory
		file, err := os.Create(dir + "/test.db")
		if err != nil {
			t.Fatal(err)
		}
		// Close the file
		file.Close()

		// Set permissions on the file to be read-only
		if err := os.Chmod(dir+"/test.db", 0400); err != nil {
			t.Fatal(err)
		}

		cfg := &Config{
			DatabaseURL: dir,
			Port:        "7000",
		}

		s := NewServer(cfg)
		if s != nil {
			t.Error("expected server to be nil")
		}
	})
}
