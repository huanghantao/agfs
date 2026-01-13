package devfs

import (
	"errors"
	"io"
	"time"

	"github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin/config"
)

const (
	PluginName = "devfs"
)

// DevFSPlugin is a minimal plugin that provides device files
type DevFSPlugin struct{}

// NewDevFSPlugin creates a new DevFS plugin
func NewDevFSPlugin() *DevFSPlugin {
	return &DevFSPlugin{}
}

func (p *DevFSPlugin) Name() string {
	return PluginName
}

func (p *DevFSPlugin) Validate(cfg map[string]interface{}) error {
	// Only mount_path is allowed (injected by framework)
	allowedKeys := []string{"mount_path"}
	return config.ValidateOnlyKnownKeys(cfg, allowedKeys)
}

func (p *DevFSPlugin) Initialize(config map[string]interface{}) error {
	return nil
}

func (p *DevFSPlugin) GetFileSystem() filesystem.FileSystem {
	return &DevFS{}
}

func (p *DevFSPlugin) GetReadme() string {
	return `DevFS Plugin - Device File System

This plugin provides standard Unix device files.

AVAILABLE DEVICES:
  /dev/null  - Null device (discards writes, returns EOF on reads)

USAGE:
  Read from /dev/null:
    cat /dev/null

  Write to /dev/null (data is discarded):
    echo "test" > /dev/null

  Use as redirect target:
    command > /dev/null 2>&1

CHARACTERISTICS:
  - /dev/null always exists
  - Reads always return EOF immediately
  - Writes are accepted and discarded
  - Cannot be deleted, renamed, or modified

VERSION: 1.0.0
`
}

func (p *DevFSPlugin) GetConfigParams() []plugin.ConfigParameter {
	return []plugin.ConfigParameter{}
}

func (p *DevFSPlugin) Shutdown() error {
	return nil
}

// DevFS is a minimal filesystem that provides device files
type DevFS struct{}

func (fs *DevFS) Read(path string, offset int64, size int64) ([]byte, error) {
	if path == "/null" {
		// Reading from /dev/null always returns EOF
		return nil, io.EOF
	}
	return nil, filesystem.ErrNotFound
}

func (fs *DevFS) Write(path string, data []byte, offset int64, flags filesystem.WriteFlag) (int64, error) {
	if path == "/null" {
		// Writing to /dev/null succeeds but discards data
		return int64(len(data)), nil
	}
	return 0, errors.New("read-only filesystem")
}

func (fs *DevFS) Stat(path string) (*filesystem.FileInfo, error) {
	if path == "/null" {
		return &filesystem.FileInfo{
			Name:    "null",
			Size:    0,
			Mode:    0666, // rw-rw-rw-
			ModTime: time.Now(),
			IsDir:   false,
			Meta:    filesystem.MetaData{Name: PluginName, Type: "device"},
		}, nil
	}
	if path == "/" {
		return &filesystem.FileInfo{
			Name:    "/",
			Size:    0,
			Mode:    0555, // r-xr-xr-x
			ModTime: time.Now(),
			IsDir:   true,
			Meta:    filesystem.MetaData{Name: PluginName, Type: "directory"},
		}, nil
	}
	return nil, filesystem.ErrNotFound
}

func (fs *DevFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	if path == "/" {
		return []filesystem.FileInfo{
			{
				Name:    "null",
				Size:    0,
				Mode:    0666,
				ModTime: time.Now(),
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: "device"},
			},
		}, nil
	}
	return nil, errors.New("not a directory")
}

func (fs *DevFS) Open(path string) (io.ReadCloser, error) {
	if path == "/null" {
		return &nullReader{}, nil
	}
	return nil, filesystem.ErrNotFound
}

func (fs *DevFS) OpenWrite(path string) (io.WriteCloser, error) {
	if path == "/null" {
		return &nullWriter{}, nil
	}
	return nil, errors.New("read-only filesystem")
}

// Unsupported write operations
func (fs *DevFS) Create(path string) error {
	return errors.New("read-only filesystem")
}

func (fs *DevFS) Mkdir(path string, perm uint32) error {
	return errors.New("read-only filesystem")
}

func (fs *DevFS) Remove(path string) error {
	return errors.New("read-only filesystem")
}

func (fs *DevFS) RemoveAll(path string) error {
	return errors.New("read-only filesystem")
}

func (fs *DevFS) Rename(oldPath, newPath string) error {
	return errors.New("read-only filesystem")
}

func (fs *DevFS) Chmod(path string, mode uint32) error {
	return errors.New("read-only filesystem")
}

// Truncate is a no-op for devfs
func (fs *DevFS) Truncate(path string, size int64) error {
	if path == "/null" {
		// Truncating /dev/null is allowed and does nothing
		return nil
	}
	return errors.New("read-only filesystem")
}

// nullReader implements io.ReadCloser for /dev/null reads
type nullReader struct{}

func (nr *nullReader) Read(p []byte) (int, error) {
	// Always return EOF immediately
	return 0, io.EOF
}

func (nr *nullReader) Close() error {
	return nil
}

// nullWriter implements io.WriteCloser for /dev/null writes
type nullWriter struct{}

func (nw *nullWriter) Write(p []byte) (int, error) {
	// Accept all data and discard it
	return len(p), nil
}

func (nw *nullWriter) Close() error {
	return nil
}

// Ensure DevFSPlugin implements ServicePlugin
var _ plugin.ServicePlugin = (*DevFSPlugin)(nil)
var _ filesystem.FileSystem = (*DevFS)(nil)
