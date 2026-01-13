package devfs

import (
	"io"
	"testing"

	"github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
)

func TestDevFSRead(t *testing.T) {
	fs := &DevFS{}

	// Reading from /null should return EOF
	data, err := fs.Read("/null", 0, 1024)
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got %v", err)
	}
	if data != nil {
		t.Errorf("Expected nil data, got %v", data)
	}

	// Reading from non-existent file should return ErrNotFound
	_, err = fs.Read("/nonexistent", 0, 1024)
	if err != filesystem.ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestDevFSWrite(t *testing.T) {
	fs := &DevFS{}

	// Writing to /null should succeed and discard data
	testData := []byte("test data")
	n, err := fs.Write("/null", testData, 0, 0)
	if err != nil {
		t.Errorf("Write to /null failed: %v", err)
	}
	if n != int64(len(testData)) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
	}

	// Writing to non-existent file should fail
	_, err = fs.Write("/nonexistent", testData, 0, 0)
	if err == nil {
		t.Error("Expected error writing to non-existent file")
	}
}

func TestDevFSStat(t *testing.T) {
	fs := &DevFS{}

	// Stat /null
	info, err := fs.Stat("/null")
	if err != nil {
		t.Errorf("Stat /null failed: %v", err)
	}
	if info.Name != "null" {
		t.Errorf("Expected name 'null', got '%s'", info.Name)
	}
	if info.Mode != 0666 {
		t.Errorf("Expected mode 0666, got %o", info.Mode)
	}
	if info.IsDir {
		t.Error("Expected /null to not be a directory")
	}

	// Stat root
	info, err = fs.Stat("/")
	if err != nil {
		t.Errorf("Stat / failed: %v", err)
	}
	if !info.IsDir {
		t.Error("Expected / to be a directory")
	}

	// Stat non-existent
	_, err = fs.Stat("/nonexistent")
	if err != filesystem.ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestDevFSReadDir(t *testing.T) {
	fs := &DevFS{}

	// ReadDir root should return null
	entries, err := fs.ReadDir("/")
	if err != nil {
		t.Errorf("ReadDir / failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
	if len(entries) > 0 && entries[0].Name != "null" {
		t.Errorf("Expected entry 'null', got '%s'", entries[0].Name)
	}

	// ReadDir non-directory should fail
	_, err = fs.ReadDir("/null")
	if err == nil {
		t.Error("Expected error reading directory from /null")
	}
}

func TestDevFSOpen(t *testing.T) {
	fs := &DevFS{}

	// Open /null for reading
	reader, err := fs.Open("/null")
	if err != nil {
		t.Errorf("Open /null failed: %v", err)
	}
	defer reader.Close()

	// Reading should return EOF
	buf := make([]byte, 10)
	n, err := reader.Read(buf)
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes read, got %d", n)
	}

	// Open non-existent should fail
	_, err = fs.Open("/nonexistent")
	if err != filesystem.ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestDevFSOpenWrite(t *testing.T) {
	fs := &DevFS{}

	// OpenWrite /null
	writer, err := fs.OpenWrite("/null")
	if err != nil {
		t.Errorf("OpenWrite /null failed: %v", err)
	}
	defer writer.Close()

	// Writing should succeed and discard data
	testData := []byte("test data")
	n, err := writer.Write(testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
	}

	// OpenWrite non-existent should fail
	_, err = fs.OpenWrite("/nonexistent")
	if err == nil {
		t.Error("Expected error opening non-existent file for writing")
	}
}

func TestDevFSReadOnlyOperations(t *testing.T) {
	fs := &DevFS{}

	// All modification operations should fail
	if err := fs.Create("/test"); err == nil {
		t.Error("Expected Create to fail")
	}

	if err := fs.Mkdir("/testdir", 0755); err == nil {
		t.Error("Expected Mkdir to fail")
	}

	if err := fs.Remove("/null"); err == nil {
		t.Error("Expected Remove to fail")
	}

	if err := fs.RemoveAll("/"); err == nil {
		t.Error("Expected RemoveAll to fail")
	}

	if err := fs.Rename("/null", "/null2"); err == nil {
		t.Error("Expected Rename to fail")
	}

	if err := fs.Chmod("/null", 0777); err == nil {
		t.Error("Expected Chmod to fail")
	}
}

func TestDevFSTruncate(t *testing.T) {
	fs := &DevFS{}

	// Truncate /null should succeed (no-op)
	err := fs.Truncate("/null", 0)
	if err != nil {
		t.Errorf("Truncate /null failed: %v", err)
	}

	// Truncate other files should fail
	err = fs.Truncate("/nonexistent", 0)
	if err == nil {
		t.Error("Expected Truncate on non-existent file to fail")
	}
}

func TestDevFSPlugin(t *testing.T) {
	plugin := NewDevFSPlugin()

	if plugin.Name() != "devfs" {
		t.Errorf("Expected plugin name 'devfs', got '%s'", plugin.Name())
	}

	// Validate should accept mount_path
	config := map[string]interface{}{
		"mount_path": "/dev",
	}
	if err := plugin.Validate(config); err != nil {
		t.Errorf("Validate failed: %v", err)
	}

	// Initialize should succeed
	if err := plugin.Initialize(config); err != nil {
		t.Errorf("Initialize failed: %v", err)
	}

	// GetFileSystem should return DevFS
	fs := plugin.GetFileSystem()
	if fs == nil {
		t.Error("GetFileSystem returned nil")
	}

	// GetReadme should return non-empty string
	readme := plugin.GetReadme()
	if readme == "" {
		t.Error("GetReadme returned empty string")
	}

	// Shutdown should succeed
	if err := plugin.Shutdown(); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}
