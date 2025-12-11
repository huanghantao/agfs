package filesystem

import (
	"io"
	"sync"
	"testing"
	"time"
)

// mockFileHandle is a mock implementation of FileHandle for testing
type mockFileHandle struct {
	id       string
	path     string
	flags    OpenFlag
	pos      int64
	data     []byte
	closed   bool
	mu       sync.Mutex
	closeCh  chan struct{}
}

func newMockFileHandle(id, path string, flags OpenFlag) *mockFileHandle {
	return &mockFileHandle{
		id:      id,
		path:    path,
		flags:   flags,
		data:    []byte{},
		closeCh: make(chan struct{}),
	}
}

func (m *mockFileHandle) ID() string   { return m.id }
func (m *mockFileHandle) Path() string { return m.path }
func (m *mockFileHandle) Flags() OpenFlag { return m.flags }

func (m *mockFileHandle) Read(buf []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	if m.pos >= int64(len(m.data)) {
		return 0, io.EOF
	}
	n := copy(buf, m.data[m.pos:])
	m.pos += int64(n)
	return n, nil
}

func (m *mockFileHandle) ReadAt(buf []byte, offset int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	if offset >= int64(len(m.data)) {
		return 0, io.EOF
	}
	n := copy(buf, m.data[offset:])
	return n, nil
}

func (m *mockFileHandle) Write(data []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	m.data = append(m.data[:m.pos], data...)
	m.pos += int64(len(data))
	return len(data), nil
}

func (m *mockFileHandle) WriteAt(data []byte, offset int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	// Extend data if needed
	needed := offset + int64(len(data))
	if needed > int64(len(m.data)) {
		m.data = append(m.data, make([]byte, needed-int64(len(m.data)))...)
	}
	copy(m.data[offset:], data)
	return len(data), nil
}

func (m *mockFileHandle) Seek(offset int64, whence int) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = m.pos + offset
	case io.SeekEnd:
		newPos = int64(len(m.data)) + offset
	}
	if newPos < 0 {
		return m.pos, io.ErrUnexpectedEOF
	}
	m.pos = newPos
	return m.pos, nil
}

func (m *mockFileHandle) Sync() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return io.ErrClosedPipe
	}
	return nil
}

func (m *mockFileHandle) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	close(m.closeCh)
	return nil
}

func (m *mockFileHandle) Stat() (*FileInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &FileInfo{
		Name:  m.path,
		Size:  int64(len(m.data)),
		Mode:  0644,
		IsDir: false,
	}, nil
}

func (m *mockFileHandle) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func TestHandleManagerConfig(t *testing.T) {
	config := DefaultHandleManagerConfig()

	if config.DefaultLease != 60*time.Second {
		t.Errorf("DefaultLease: got %v, want 60s", config.DefaultLease)
	}
	if config.MaxLease != 5*time.Minute {
		t.Errorf("MaxLease: got %v, want 5min", config.MaxLease)
	}
	if config.MaxHandles != 10000 {
		t.Errorf("MaxHandles: got %d, want 10000", config.MaxHandles)
	}
	if config.CleanupInterval != 10*time.Second {
		t.Errorf("CleanupInterval: got %v, want 10s", config.CleanupInterval)
	}
}

func TestHandleManagerRegister(t *testing.T) {
	config := HandleManagerConfig{
		DefaultLease:    100 * time.Millisecond,
		MaxLease:        500 * time.Millisecond,
		MaxHandles:      10,
		CleanupInterval: 50 * time.Millisecond,
	}
	hm := NewHandleManager(config)
	defer hm.Stop()

	handle := newMockFileHandle("test-1", "/test/file.txt", O_RDWR)
	id, expiresAt, err := hm.Register(handle, "/test/file.txt", O_RDWR, 0)

	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if id == "" {
		t.Error("Expected non-empty handle ID")
	}
	if id[:2] != "h_" {
		t.Errorf("Handle ID should start with 'h_', got %s", id)
	}
	if expiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
	if hm.Count() != 1 {
		t.Errorf("Count: got %d, want 1", hm.Count())
	}
}

func TestHandleManagerGet(t *testing.T) {
	config := HandleManagerConfig{
		DefaultLease:    100 * time.Millisecond,
		MaxLease:        500 * time.Millisecond,
		MaxHandles:      10,
		CleanupInterval: 50 * time.Millisecond,
	}
	hm := NewHandleManager(config)
	defer hm.Stop()

	originalHandle := newMockFileHandle("test-1", "/test/file.txt", O_RDWR)
	id, _, err := hm.Register(originalHandle, "/test/file.txt", O_RDWR, 0)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Get should succeed
	retrievedHandle, err := hm.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrievedHandle != originalHandle {
		t.Error("Retrieved handle should be the same as original")
	}

	// Get non-existent handle should fail
	_, err = hm.Get("non-existent")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestHandleManagerLeaseRefresh(t *testing.T) {
	config := HandleManagerConfig{
		DefaultLease:    100 * time.Millisecond,
		MaxLease:        500 * time.Millisecond,
		MaxHandles:      10,
		CleanupInterval: 1 * time.Second, // Long cleanup interval to avoid interference
	}
	hm := NewHandleManager(config)
	defer hm.Stop()

	handle := newMockFileHandle("test-1", "/test/file.txt", O_RDWR)
	id, originalExpiry, err := hm.Register(handle, "/test/file.txt", O_RDWR, 0)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Wait a bit
	time.Sleep(30 * time.Millisecond)

	// Get should refresh the lease
	_, err = hm.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	info, err := hm.GetInfo(id)
	if err != nil {
		t.Fatalf("GetInfo failed: %v", err)
	}

	if !info.ExpiresAt.After(originalExpiry) {
		t.Error("Lease should have been refreshed")
	}
}

func TestHandleManagerLeaseExpiry(t *testing.T) {
	config := HandleManagerConfig{
		DefaultLease:    50 * time.Millisecond,
		MaxLease:        100 * time.Millisecond,
		MaxHandles:      10,
		CleanupInterval: 20 * time.Millisecond,
	}
	hm := NewHandleManager(config)
	defer hm.Stop()

	handle := newMockFileHandle("test-1", "/test/file.txt", O_RDWR)
	id, _, err := hm.Register(handle, "/test/file.txt", O_RDWR, 0)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Wait for lease to expire
	time.Sleep(100 * time.Millisecond)

	// Get should fail after expiry
	_, err = hm.Get(id)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound after expiry, got %v", err)
	}

	// Handle should be closed
	if !handle.IsClosed() {
		t.Error("Handle should be closed after expiry")
	}
}

func TestHandleManagerRenew(t *testing.T) {
	config := HandleManagerConfig{
		DefaultLease:    100 * time.Millisecond,
		MaxLease:        500 * time.Millisecond,
		MaxHandles:      10,
		CleanupInterval: 1 * time.Second,
	}
	hm := NewHandleManager(config)
	defer hm.Stop()

	handle := newMockFileHandle("test-1", "/test/file.txt", O_RDWR)
	id, _, err := hm.Register(handle, "/test/file.txt", O_RDWR, 0)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Renew with longer lease
	newExpiry, err := hm.Renew(id, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Renew failed: %v", err)
	}
	if newExpiry.Before(time.Now().Add(150 * time.Millisecond)) {
		t.Error("New expiry should be at least 150ms in the future")
	}

	// Renew non-existent handle should fail
	_, err = hm.Renew("non-existent", 100*time.Millisecond)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestHandleManagerRenewCapsToMaxLease(t *testing.T) {
	config := HandleManagerConfig{
		DefaultLease:    100 * time.Millisecond,
		MaxLease:        200 * time.Millisecond,
		MaxHandles:      10,
		CleanupInterval: 1 * time.Second,
	}
	hm := NewHandleManager(config)
	defer hm.Stop()

	handle := newMockFileHandle("test-1", "/test/file.txt", O_RDWR)
	id, _, err := hm.Register(handle, "/test/file.txt", O_RDWR, 0)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Try to renew with lease longer than MaxLease
	newExpiry, err := hm.Renew(id, 1*time.Hour)
	if err != nil {
		t.Fatalf("Renew failed: %v", err)
	}

	// Should be capped to MaxLease
	expectedMax := time.Now().Add(config.MaxLease + 50*time.Millisecond)
	if newExpiry.After(expectedMax) {
		t.Error("Lease should be capped to MaxLease")
	}
}

func TestHandleManagerClose(t *testing.T) {
	config := HandleManagerConfig{
		DefaultLease:    100 * time.Millisecond,
		MaxLease:        500 * time.Millisecond,
		MaxHandles:      10,
		CleanupInterval: 1 * time.Second,
	}
	hm := NewHandleManager(config)
	defer hm.Stop()

	handle := newMockFileHandle("test-1", "/test/file.txt", O_RDWR)
	id, _, err := hm.Register(handle, "/test/file.txt", O_RDWR, 0)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Close the handle
	err = hm.Close(id)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Handle should be closed
	if !handle.IsClosed() {
		t.Error("Handle should be closed")
	}

	// Get should fail
	_, err = hm.Get(id)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound after close, got %v", err)
	}

	// Close non-existent should fail
	err = hm.Close("non-existent")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestHandleManagerMaxHandles(t *testing.T) {
	config := HandleManagerConfig{
		DefaultLease:    100 * time.Millisecond,
		MaxLease:        500 * time.Millisecond,
		MaxHandles:      3,
		CleanupInterval: 1 * time.Second,
	}
	hm := NewHandleManager(config)
	defer hm.Stop()

	// Register max handles
	for i := 0; i < 3; i++ {
		handle := newMockFileHandle("test", "/test/file.txt", O_RDWR)
		_, _, err := hm.Register(handle, "/test/file.txt", O_RDWR, 0)
		if err != nil {
			t.Fatalf("Register %d failed: %v", i, err)
		}
	}

	// Next register should fail
	handle := newMockFileHandle("test", "/test/file.txt", O_RDWR)
	_, _, err := hm.Register(handle, "/test/file.txt", O_RDWR, 0)
	if err == nil {
		t.Error("Expected error when exceeding MaxHandles")
	}
}

func TestHandleManagerList(t *testing.T) {
	config := HandleManagerConfig{
		DefaultLease:    100 * time.Millisecond,
		MaxLease:        500 * time.Millisecond,
		MaxHandles:      10,
		CleanupInterval: 1 * time.Second,
	}
	hm := NewHandleManager(config)
	defer hm.Stop()

	// Register multiple handles
	for i := 0; i < 3; i++ {
		handle := newMockFileHandle("test", "/test/file.txt", O_RDWR)
		_, _, err := hm.Register(handle, "/test/file.txt", O_RDWR, 0)
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	list := hm.List()
	if len(list) != 3 {
		t.Errorf("List length: got %d, want 3", len(list))
	}
}

func TestHandleManagerCleanup(t *testing.T) {
	config := HandleManagerConfig{
		DefaultLease:    30 * time.Millisecond,
		MaxLease:        100 * time.Millisecond,
		MaxHandles:      10,
		CleanupInterval: 20 * time.Millisecond,
	}
	hm := NewHandleManager(config)
	defer hm.Stop()

	handles := make([]*mockFileHandle, 3)
	for i := 0; i < 3; i++ {
		handles[i] = newMockFileHandle("test", "/test/file.txt", O_RDWR)
		_, _, err := hm.Register(handles[i], "/test/file.txt", O_RDWR, 0)
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	if hm.Count() != 3 {
		t.Errorf("Count before cleanup: got %d, want 3", hm.Count())
	}

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// All handles should be cleaned up
	if hm.Count() != 0 {
		t.Errorf("Count after cleanup: got %d, want 0", hm.Count())
	}

	// All handles should be closed
	for i, h := range handles {
		if !h.IsClosed() {
			t.Errorf("Handle %d should be closed", i)
		}
	}
}

func TestHandleManagerStop(t *testing.T) {
	config := HandleManagerConfig{
		DefaultLease:    1 * time.Second,
		MaxLease:        5 * time.Second,
		MaxHandles:      10,
		CleanupInterval: 100 * time.Millisecond,
	}
	hm := NewHandleManager(config)

	handles := make([]*mockFileHandle, 3)
	for i := 0; i < 3; i++ {
		handles[i] = newMockFileHandle("test", "/test/file.txt", O_RDWR)
		_, _, err := hm.Register(handles[i], "/test/file.txt", O_RDWR, 0)
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	// Stop should close all handles
	hm.Stop()

	// All handles should be closed
	for i, h := range handles {
		if !h.IsClosed() {
			t.Errorf("Handle %d should be closed after Stop", i)
		}
	}

	if hm.Count() != 0 {
		t.Errorf("Count after Stop: got %d, want 0", hm.Count())
	}
}

func TestHandleManagerConcurrency(t *testing.T) {
	config := HandleManagerConfig{
		DefaultLease:    500 * time.Millisecond,
		MaxLease:        1 * time.Second,
		MaxHandles:      100,
		CleanupInterval: 100 * time.Millisecond,
	}
	hm := NewHandleManager(config)
	defer hm.Stop()

	var wg sync.WaitGroup
	ids := make(chan string, 50)

	// Concurrent registrations
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handle := newMockFileHandle("test", "/test/file.txt", O_RDWR)
			id, _, err := hm.Register(handle, "/test/file.txt", O_RDWR, 0)
			if err != nil {
				t.Errorf("Register failed: %v", err)
				return
			}
			ids <- id
		}()
	}

	wg.Wait()
	close(ids)

	// Verify all registrations succeeded
	registeredIDs := make([]string, 0)
	for id := range ids {
		registeredIDs = append(registeredIDs, id)
	}

	if len(registeredIDs) != 50 {
		t.Errorf("Expected 50 registered handles, got %d", len(registeredIDs))
	}

	// Concurrent gets and renews
	for _, id := range registeredIDs {
		wg.Add(2)
		go func(id string) {
			defer wg.Done()
			_, _ = hm.Get(id)
		}(id)
		go func(id string) {
			defer wg.Done()
			_, _ = hm.Renew(id, 200*time.Millisecond)
		}(id)
	}

	wg.Wait()
}
