package filesystem

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// ManagedHandle wraps a FileHandle with lease management metadata
type ManagedHandle struct {
	Handle     FileHandle
	Path       string
	Flags      OpenFlag
	LeaseTime  time.Duration // Lease duration
	ExpiresAt  time.Time     // When the lease expires
	CreatedAt  time.Time
	LastAccess time.Time
}

// HandleManagerConfig configures the HandleManager behavior
type HandleManagerConfig struct {
	DefaultLease    time.Duration // Default lease duration (default: 60s)
	MaxLease        time.Duration // Maximum lease duration (default: 5min)
	MaxHandles      int           // Maximum number of concurrent handles (default: 10000)
	CleanupInterval time.Duration // How often to clean up expired handles (default: 10s)
}

// DefaultHandleManagerConfig returns the default configuration
func DefaultHandleManagerConfig() HandleManagerConfig {
	return HandleManagerConfig{
		DefaultLease:    60 * time.Second,
		MaxLease:        5 * time.Minute,
		MaxHandles:      10000,
		CleanupInterval: 10 * time.Second,
	}
}

// HandleManager manages file handles with lease-based lifecycle
type HandleManager struct {
	handles map[string]*ManagedHandle
	mu      sync.RWMutex
	config  HandleManagerConfig
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewHandleManager creates a new HandleManager with the given configuration
func NewHandleManager(config HandleManagerConfig) *HandleManager {
	if config.DefaultLease == 0 {
		config.DefaultLease = 60 * time.Second
	}
	if config.MaxLease == 0 {
		config.MaxLease = 5 * time.Minute
	}
	if config.MaxHandles == 0 {
		config.MaxHandles = 10000
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 10 * time.Second
	}

	hm := &HandleManager{
		handles: make(map[string]*ManagedHandle),
		config:  config,
		stopCh:  make(chan struct{}),
	}

	// Start background cleanup goroutine
	hm.wg.Add(1)
	go hm.cleanupLoop()

	return hm
}

// generateID generates a unique handle ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "h_" + hex.EncodeToString(b)
}

// Register registers a new handle with the manager
// Returns the handle ID and the expiration time
func (hm *HandleManager) Register(handle FileHandle, path string, flags OpenFlag, lease time.Duration) (string, time.Time, error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Check capacity
	if len(hm.handles) >= hm.config.MaxHandles {
		return "", time.Time{}, fmt.Errorf("maximum number of handles (%d) reached", hm.config.MaxHandles)
	}

	// Validate and cap lease duration
	if lease <= 0 {
		lease = hm.config.DefaultLease
	}
	if lease > hm.config.MaxLease {
		lease = hm.config.MaxLease
	}

	now := time.Now()
	expiresAt := now.Add(lease)

	id := generateID()
	hm.handles[id] = &ManagedHandle{
		Handle:     handle,
		Path:       path,
		Flags:      flags,
		LeaseTime:  lease,
		ExpiresAt:  expiresAt,
		CreatedAt:  now,
		LastAccess: now,
	}

	log.Debugf("[HandleManager] Registered handle %s for %s, expires at %s", id, path, expiresAt.Format(time.RFC3339))
	return id, expiresAt, nil
}

// Get retrieves a handle by ID and refreshes its lease
func (hm *HandleManager) Get(id string) (FileHandle, error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	mh, exists := hm.handles[id]
	if !exists {
		return nil, ErrNotFound
	}

	now := time.Now()
	if now.After(mh.ExpiresAt) {
		// Handle has expired, clean it up
		mh.Handle.Close()
		delete(hm.handles, id)
		return nil, ErrNotFound
	}

	// Refresh lease on access
	mh.LastAccess = now
	mh.ExpiresAt = now.Add(mh.LeaseTime)

	return mh.Handle, nil
}

// GetInfo retrieves handle information without refreshing the lease
func (hm *HandleManager) GetInfo(id string) (*ManagedHandle, error) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	mh, exists := hm.handles[id]
	if !exists {
		return nil, ErrNotFound
	}

	if time.Now().After(mh.ExpiresAt) {
		return nil, ErrNotFound
	}

	return mh, nil
}

// Renew extends the lease of a handle
func (hm *HandleManager) Renew(id string, lease time.Duration) (time.Time, error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	mh, exists := hm.handles[id]
	if !exists {
		return time.Time{}, ErrNotFound
	}

	now := time.Now()
	if now.After(mh.ExpiresAt) {
		// Handle has expired
		mh.Handle.Close()
		delete(hm.handles, id)
		return time.Time{}, ErrNotFound
	}

	// Validate and cap lease duration
	if lease <= 0 {
		lease = mh.LeaseTime
	}
	if lease > hm.config.MaxLease {
		lease = hm.config.MaxLease
	}

	mh.LeaseTime = lease
	mh.ExpiresAt = now.Add(lease)
	mh.LastAccess = now

	log.Debugf("[HandleManager] Renewed handle %s, new expiry: %s", id, mh.ExpiresAt.Format(time.RFC3339))
	return mh.ExpiresAt, nil
}

// Close closes and removes a handle
func (hm *HandleManager) Close(id string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	mh, exists := hm.handles[id]
	if !exists {
		return ErrNotFound
	}

	err := mh.Handle.Close()
	delete(hm.handles, id)

	log.Debugf("[HandleManager] Closed handle %s", id)
	return err
}

// List returns information about all active handles
func (hm *HandleManager) List() []*ManagedHandle {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	now := time.Now()
	result := make([]*ManagedHandle, 0, len(hm.handles))
	for _, mh := range hm.handles {
		if now.Before(mh.ExpiresAt) {
			result = append(result, mh)
		}
	}
	return result
}

// Count returns the number of active handles
func (hm *HandleManager) Count() int {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return len(hm.handles)
}

// Config returns the current configuration
func (hm *HandleManager) Config() HandleManagerConfig {
	return hm.config
}

// cleanupLoop periodically removes expired handles
func (hm *HandleManager) cleanupLoop() {
	defer hm.wg.Done()

	ticker := time.NewTicker(hm.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hm.cleanup()
		case <-hm.stopCh:
			return
		}
	}
}

// cleanup removes all expired handles
func (hm *HandleManager) cleanup() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	now := time.Now()
	expired := 0

	for id, mh := range hm.handles {
		if now.After(mh.ExpiresAt) {
			mh.Handle.Close()
			delete(hm.handles, id)
			expired++
		}
	}

	if expired > 0 {
		log.Debugf("[HandleManager] Cleaned up %d expired handles, %d remaining", expired, len(hm.handles))
	}
}

// Stop stops the background cleanup goroutine and closes all handles
func (hm *HandleManager) Stop() {
	close(hm.stopCh)
	hm.wg.Wait()

	// Close all remaining handles
	hm.mu.Lock()
	defer hm.mu.Unlock()

	for id, mh := range hm.handles {
		mh.Handle.Close()
		delete(hm.handles, id)
	}

	log.Info("[HandleManager] Stopped and cleaned up all handles")
}
