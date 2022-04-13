package cache

import (
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
)

var cache *Ristretto
var onceCache sync.Once

// RistrettoCacheTTL contains the default time after which a key-value in cache pair will expire
const RistrettoCacheTTL = 60 * time.Minute

// Ristretto is the final data type used when dealing with cache
type Ristretto struct {
	Cacheable
}

// Cacheable includes all the functions a struct must implement in order for it to be embedded in Ristretto
type Cacheable interface {
	Get(string) (interface{}, bool)
	Set(string, interface{}, time.Duration) bool
	Del(string)
	Clear()
}

// Because otherwise we cannot mock cache for tests
type storage struct {
	cache *ristretto.Cache
}

// NewRistrettoCache initializes a cache
var NewRistrettoCache = func() (*Ristretto, error) {
	var err error
	onceCache.Do(func() {
		var ristrettoCache *ristretto.Cache
		ristrettoCache, err = ristretto.NewCache(&ristretto.Config{
			NumCounters: 1e8,     // Num keys to track frequency of (100M).
			MaxCost:     2 << 30, // Maximum cost of cache (2GB).
			BufferItems: 64,      // Number of keys per Get buffer.
		})

		if err == nil {
			cache = &Ristretto{Cacheable: &storage{ristrettoCache}}
		}
	})

	return cache, err
}

// Get returns item behind key "key" and a boolean representing whether the item was found or not
func (s *storage) Get(key string) (interface{}, bool) {
	return s.cache.Get(key)
}

// Set stores data to cache with specific key and ttl. If ttl == -1, RistrettoCacheTTL will be used.
func (s *storage) Set(key string, value interface{}, ttl time.Duration) bool {
	if ttl == -1 {
		ttl = RistrettoCacheTTL
	}
	return s.cache.SetWithTTL(key, value, 1, ttl)
}

// Del deletes item with key "key" from cache
func (s *storage) Del(key string) {
	s.cache.Del(key)
}

func (s *storage) Clear() {
	s.cache.Clear()
}
