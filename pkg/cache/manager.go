// Copyright 2025 Open3FS Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"sync"
	"time"
)

// Entry represents a cached value with expiration time
type Entry struct {
	Value      any
	ExpireTime time.Time
}

// Config defines cache configuration parameters
type Config struct {
	TTL             time.Duration
	CleanupInterval time.Duration
	Enabled         bool
}

// DefaultCacheConfig provides default cache configuration
var DefaultCacheConfig = Config{
	TTL:             5 * time.Minute,
	CleanupInterval: 10 * time.Minute,
	Enabled:         true,
}

// Manager manages multiple caches with cleanup
type Manager struct {
	caches      map[string]*sync.Map
	config      Config
	lastCleanup time.Time
	mu          sync.RWMutex

	// Public fields for direct access
	TTL     time.Duration
	Enabled bool

	// Node caches
	ServiceNodesCache sync.Map
	MetaNodesCache    sync.Map
	NodeGroupCache    sync.Map
}

// NewCacheManager creates a new Manager
func NewCacheManager(config Config) *Manager {
	cm := &Manager{
		caches:      make(map[string]*sync.Map),
		config:      config,
		lastCleanup: time.Now(),
		TTL:         config.TTL,
		Enabled:     config.Enabled,
	}

	if config.Enabled && config.CleanupInterval > 0 {
		go cm.startCleanup()
	}

	return cm
}

// Get retrieves a value from the cache
func (cm *Manager) Get(cacheName string, key any) (any, bool) {
	if !cm.config.Enabled {
		return nil, false
	}

	cm.mu.RLock()
	cache, exists := cm.caches[cacheName]
	cm.mu.RUnlock()

	if !exists {
		return nil, false
	}

	value, ok := cache.Load(key)
	if !ok {
		return nil, false
	}

	entry, ok := value.(Entry)
	if !ok {
		return value, true
	}

	if time.Now().After(entry.ExpireTime) {
		cache.Delete(key)
		return nil, false
	}

	return entry.Value, true
}

// Set stores a value in the cache
func (cm *Manager) Set(cacheName string, key, value any) {
	if !cm.config.Enabled {
		return
	}

	cm.mu.Lock()
	cache, exists := cm.caches[cacheName]
	if !exists {
		cache = &sync.Map{}
		cm.caches[cacheName] = cache
	}
	cm.mu.Unlock()

	entry := Entry{
		Value:      value,
		ExpireTime: time.Now().Add(cm.config.TTL),
	}

	cache.Store(key, entry)
}

// Delete removes a value from the cache
func (cm *Manager) Delete(cacheName string, key any) {
	cm.mu.RLock()
	cache, exists := cm.caches[cacheName]
	cm.mu.RUnlock()

	if exists {
		cache.Delete(key)
	}
}

// startCleanup starts the cache cleanup routine
func (cm *Manager) startCleanup() {
	ticker := time.NewTicker(cm.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		cm.cleanup()
	}
}

// cleanup removes expired entries from all caches
func (cm *Manager) cleanup() {
	now := time.Now()
	cm.lastCleanup = now

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, cache := range cm.caches {
		var keysToDelete []any
		cache.Range(func(key, value any) bool {
			if entry, ok := value.(Entry); ok {
				if entry.ExpireTime.Before(now) {
					keysToDelete = append(keysToDelete, key)
				}
			}
			return true
		})

		for _, key := range keysToDelete {
			cache.Delete(key)
		}
	}
}

// GetCachedNodes retrieves nodes from cache using the specified key
func (cm *Manager) GetCachedNodes(cache *sync.Map, key string) []string {
	if !cm.Enabled {
		return nil
	}

	if cached, ok := cache.Load(key); ok {
		if entry, ok := cached.(Entry); ok {
			if time.Now().Before(entry.ExpireTime) {
				if nodes, ok := entry.Value.([]string); ok {
					return nodes
				}
			}
		} else if nodes, ok := cached.([]string); ok {
			return nodes
		}
	}
	return nil
}

// CacheNodes stores nodes in cache using the specified key
func (cm *Manager) CacheNodes(cache *sync.Map, key string, nodes []string) {
	if !cm.Enabled || len(nodes) == 0 {
		return
	}

	cache.Store(key, Entry{
		Value:      nodes,
		ExpireTime: time.Now().Add(cm.TTL),
	})
}

// GetCacheKey generates a unified format key for caching
func (cm *Manager) GetCacheKey(prefix string, parts ...string) string {
	if len(parts) == 0 {
		return prefix
	}

	key := prefix
	for _, part := range parts {
		key += ":" + part
	}
	return key
}
