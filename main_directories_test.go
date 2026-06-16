package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

func TestEnsureDirectories(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "lesstruct-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	// Create a test config
	cfg := &config.Config{
		DBPath: "data/lesstruct.db",
	}

	// Ensure directories
	logger := util.NewLogger(os.Stdout)
	err = ensureDirectories(cfg, logger)
	if err != nil {
		t.Fatalf("ensureDirectories() error = %v", err)
	}

	// Verify plugins directory exists
	pluginsPath := filepath.Join(tempDir, "plugins")
	info, err := os.Stat(pluginsPath)
	if err != nil {
		t.Fatalf("Failed to stat plugins directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("plugins path is not a directory")
	}

	// Verify data directory exists
	dataPath := filepath.Join(tempDir, "data")
	info, err = os.Stat(dataPath)
	if err != nil {
		t.Fatalf("Failed to stat data directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("data path is not a directory")
	}
}

func TestEnsureDirectories_Idempotent(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "lesstruct-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	cfg := &config.Config{
		DBPath: "data/lesstruct.db",
	}

	// Create directories twice - should not error
	logger := util.NewLogger(os.Stdout)
	err = ensureDirectories(cfg, logger)
	if err != nil {
		t.Fatalf("First ensureDirectories() error = %v", err)
	}

	err = ensureDirectories(cfg, logger)
	if err != nil {
		t.Fatalf("Second ensureDirectories() error = %v", err)
	}

	// Verify directories still exist
	pluginsPath := filepath.Join(tempDir, "plugins")
	if _, err := os.Stat(pluginsPath); err != nil {
		t.Errorf("plugins directory doesn't exist after second call: %v", err)
	}
}
