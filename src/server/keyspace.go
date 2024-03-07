package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/echovault/echovault/src/utils"
	"log"
	"math/rand"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// KeyLock tries to acquire the write lock for the specified key.
// If the context passed to the function finishes before the lock is acquired, an error is returned.
func (server *Server) KeyLock(ctx context.Context, key string) (bool, error) {
	for {
		select {
		default:
			if server.keyLocks[key] == nil {
				return false, fmt.Errorf("key %s not found", key)
			}
			ok := server.keyLocks[key].TryLock()
			if ok {
				return true, nil
			}
		case <-ctx.Done():
			return false, context.Cause(ctx)
		}
	}
}

func (server *Server) KeyUnlock(key string) {
	if server.KeyExists(key) {
		server.keyLocks[key].Unlock()
	}
}

// KeyRLock tries to acquire the read lock for the specified key.
// If the context passed to the function finishes before the lock is acquired, an error is returned.
func (server *Server) KeyRLock(ctx context.Context, key string) (bool, error) {
	for {
		select {
		default:
			if server.keyLocks[key] == nil {
				return false, fmt.Errorf("key %s not found", key)
			}
			ok := server.keyLocks[key].TryRLock()
			if ok {
				return true, nil
			}
		case <-ctx.Done():
			return false, context.Cause(ctx)
		}
	}
}

func (server *Server) KeyRUnlock(key string) {
	if server.KeyExists(key) {
		server.keyLocks[key].RUnlock()
	}
}

func (server *Server) KeyExists(key string) bool {
	return server.keyLocks[key] != nil
}

// CreateKeyAndLock creates a new key lock and immediately locks it if the key does not exist.
// If the key exists, the existing key is locked.
func (server *Server) CreateKeyAndLock(ctx context.Context, key string) (bool, error) {
	if utils.IsMaxMemoryExceeded(server.Config.MaxMemory) && server.Config.EvictionPolicy == utils.NoEviction {
		return false, errors.New("max memory reached, key not created")
	}

	server.keyCreationLock.Lock()
	defer server.keyCreationLock.Unlock()

	if !server.KeyExists(key) {
		keyLock := &sync.RWMutex{}
		keyLock.Lock()
		server.keyLocks[key] = keyLock
		return true, nil
	}

	return server.KeyLock(ctx, key)
}

// GetValue retrieves the current value at the specified key.
// The key must be read-locked before calling this function.
func (server *Server) GetValue(ctx context.Context, key string) interface{} {
	err := server.updateKeyInCache(ctx, key)
	if err != nil {
		log.Printf("GetValue error: %+v\n", err)
	}
	return server.store[key]
}

// SetValue updates the value in the store at the specified key with the given value.
// If we're in not in cluster (i.e. in standalone mode), then the change count is incremented
// in the snapshot engine.
// This count triggers a snapshot when the threshold is reached.
// The key must be locked prior to calling this function.
func (server *Server) SetValue(ctx context.Context, key string, value interface{}) error {
	if utils.IsMaxMemoryExceeded(server.Config.MaxMemory) && server.Config.EvictionPolicy == utils.NoEviction {
		return errors.New("max memory reached, key value not set")
	}

	server.store[key] = value

	err := server.updateKeyInCache(ctx, key)
	if err != nil {
		log.Printf("SetValue error: %+v\n", err)
	}

	if !server.IsInCluster() {
		server.SnapshotEngine.IncrementChangeCount()
	}

	return nil
}

// The SetKeyExpiry receiver function sets the expiry time of a key.
// The key parameter represents the key whose expiry time is to be set/updated.
// The expire parameter is the new expiry time.
// The touch parameter determines whether to update the keys access count on lfu eviction policy,
// or the access time on lru eviction policy.
// The key must be locked prior to calling this function.
func (server *Server) SetKeyExpiry(ctx context.Context, key string, expire time.Time, touch bool) {
	server.keyExpiry[key] = expire
	if touch {
		err := server.updateKeyInCache(ctx, key)
		if err != nil {
			log.Printf("SetKeyExpiry error: %+v\n", err)
		}
	}
}

// RemoveKeyExpiry is called by commands that remove key expiry (e.g. PERSIST).
// The key must be locked prior ro calling this function.
func (server *Server) RemoveKeyExpiry(key string) {
	server.keyExpiry[key] = time.Time{}
	switch {
	case slices.Contains([]string{utils.AllKeysLFU, utils.VolatileLFU}, server.Config.EvictionPolicy):
		server.lfuCache.cache.Delete(key)
	case slices.Contains([]string{utils.AllKeysLRU, utils.VolatileLRU}, server.Config.EvictionPolicy):
		server.lruCache.cache.Delete(key)
	}
}

// GetState creates a deep copy of the store map.
// It is used to retrieve the current state for persistence but can also be used for other
// functions that require a deep copy of the state.
// The copy only starts when there's no current copy in progress (represented by StateCopyInProgress atomic boolean)
// and when there's no current state mutation in progress (represented by StateMutationInProgress atomic boolean)
func (server *Server) GetState() map[string]interface{} {
	for {
		if !server.StateCopyInProgress.Load() && !server.StateMutationInProgress.Load() {
			server.StateCopyInProgress.Store(true)
			break
		}
	}
	data := make(map[string]interface{})
	for k, v := range server.store {
		data[k] = v
	}
	server.StateCopyInProgress.Store(false)
	return data
}

// updateKeyInCache updates either the key access count or the most recent access time in the cache
// depending on whether an LFU or LRU strategy was used.
func (server *Server) updateKeyInCache(ctx context.Context, key string) error {
	// If max memory is 0, there's no max so no need to update caches
	if server.Config.MaxMemory == 0 {
		return nil
	}
	switch strings.ToLower(server.Config.EvictionPolicy) {
	case utils.AllKeysLFU:
		server.lfuCache.mutex.Lock()
		defer server.lfuCache.mutex.Unlock()
		server.lfuCache.cache.Update(key)
	case utils.AllKeysLRU:
		server.lruCache.mutex.Lock()
		defer server.lruCache.mutex.Unlock()
		server.lruCache.cache.Update(key)
	case utils.VolatileLFU:
		server.lfuCache.mutex.Lock()
		defer server.lfuCache.mutex.Unlock()
		if _, ok := server.keyExpiry[key]; ok {
			server.lfuCache.cache.Update(key)
		}
	case utils.VolatileLRU:
		server.lruCache.mutex.Lock()
		defer server.lruCache.mutex.Unlock()
		if _, ok := server.keyExpiry[key]; ok {
			server.lruCache.cache.Update(key)
		}
	}
	if err := server.adjustMemoryUsage(ctx); err != nil {
		return fmt.Errorf("updateKeyInCache: %+v", err)
	}
	return nil
}

func (server *Server) adjustMemoryUsage(ctx context.Context) error {
	// If max memory is 0, there's no need to adjust memory usage.
	if server.Config.MaxMemory == 0 {
		return nil
	}
	// Check if memory usage is above max-memory.
	// If it is, pop items from the cache until we get under the limit.
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	// If we're using less memory than the max-memory, there's no need to evict.
	if memStats.HeapInuse < server.Config.MaxMemory {
		return nil
	}
	// Force a garbage collection first before we start evicting key.
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	if memStats.HeapInuse < server.Config.MaxMemory {
		return nil
	}
	// We've done a GC, but we're still at or above the max memory limit.
	// Start a loop that evicts keys until either the heap is empty or
	// we're below the max memory limit.
	switch {
	case slices.Contains([]string{utils.AllKeysLFU, utils.VolatileLFU}, strings.ToLower(server.Config.EvictionPolicy)):
		// Remove keys from LFU cache until we're below the max memory limit or
		// until the LFU cache is empty.
		server.lfuCache.mutex.Lock()
		defer server.lfuCache.mutex.Unlock()
		for {
			// Return if cache is empty
			if server.lfuCache.cache.Len() == 0 {
				return fmt.Errorf("adjsutMemoryUsage -> LFU cache empty")
			}
			key := server.lfuCache.cache.Pop().(string)
			if _, err := server.KeyLock(ctx, key); err != nil {
				return fmt.Errorf("adjustMemoryUsage -> LFU cache eviction: %+v", err)
			}
			// Delete the keys
			delete(server.store, key)
			delete(server.keyExpiry, key)
			delete(server.keyLocks, key)
			// Run garbage collection
			runtime.GC()
			// Return if we're below max memory
			runtime.ReadMemStats(&memStats)
			if memStats.HeapInuse < server.Config.MaxMemory {
				return nil
			}
		}
	case slices.Contains([]string{utils.AllKeysLRU, utils.VolatileLRU}, strings.ToLower(server.Config.EvictionPolicy)):
		// Remove keys from th LRU cache until we're below the max memory limit or
		// until the LRU cache is empty.
		server.lruCache.mutex.Lock()
		defer server.lruCache.mutex.Unlock()
		for {
			// Return if cache is empty
			if server.lruCache.cache.Len() == 0 {
				return fmt.Errorf("adjsutMemoryUsage -> LRU cache empty")
			}
			key := server.lruCache.cache.Pop().(string)
			if _, err := server.KeyLock(ctx, key); err != nil {
				return fmt.Errorf("adjustMemoryUsage -> LRU cache eviction: %+v", err)
			}
			// Delete the keys
			delete(server.store, key)
			delete(server.keyExpiry, key)
			delete(server.keyLocks, key)
			// Run garbage collection
			runtime.GC()
			// Return if we're below max memory
			runtime.ReadMemStats(&memStats)
			if memStats.HeapInuse < server.Config.MaxMemory {
				return nil
			}
		}
	case slices.Contains([]string{utils.AllKeysRandom}, strings.ToLower(server.Config.EvictionPolicy)):
		// Remove random keys until we're below the max memory limit
		// or there are no more keys remaining.
		for {
			// If there are no keys, return error
			if len(server.keyLocks) == 0 {
				err := errors.New("no keys to evict")
				return fmt.Errorf("adjustMemoryUsage -> all keys random: %+v", err)
			}
			// Get random key
			idx := rand.Intn(len(server.keyLocks))
			for key, _ := range server.keyLocks {
				if idx == 0 {
					// Lock the key
					if _, err := server.KeyLock(ctx, key); err != nil {
						return fmt.Errorf("adjustMemoryUsage -> all keys random: %+v", err)
					}
					// Delete the key
					delete(server.keyLocks, key)
					delete(server.store, key)
					delete(server.keyExpiry, key)
					// Run garbage collection
					runtime.GC()
					// Return if we're below max memory
					runtime.ReadMemStats(&memStats)
					if memStats.HeapInuse < server.Config.MaxMemory {
						return nil
					}
				}
				idx--
			}
		}
	case slices.Contains([]string{utils.VolatileRandom}, strings.ToLower(server.Config.EvictionPolicy)):
		// Remove random keys with expiry time until we're below the max memory limit
		// or there are no more keys with expiry time.
		for {
			// If there are no volatile keys, return error
			if len(server.keyExpiry) == 0 {
				err := errors.New("no volatile keys to evict")
				return fmt.Errorf("adjustMemoryUsage -> volatile keys random: %+v", err)
			}
			// Get random volatile key
			idx := rand.Intn(len(server.keyExpiry))
			for key, _ := range server.keyExpiry {
				if idx == 0 {
					// Lock the key
					if _, err := server.KeyLock(ctx, key); err != nil {
						return fmt.Errorf("adjustMemoryUsage -> volatile keys random: %+v", err)
					}
					// Delete the key
					delete(server.keyLocks, key)
					delete(server.store, key)
					delete(server.keyExpiry, key)
					// Run garbage collection
					runtime.GC()
					// Return if we're below max memory
					runtime.ReadMemStats(&memStats)
					if memStats.HeapInuse < server.Config.MaxMemory {
						return nil
					}
				}
				idx--
			}
		}
	default:
		return nil
	}
}
