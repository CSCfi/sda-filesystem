package cache

import (
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
)

var cache *Ristretto
var onceCache sync.Once

const RistrettoCacheTTL = 60 * time.Minute

type Ristretto struct {
	cache *ristretto.Cache
}

// NewRistrettoCache initializes a cache
func NewRistrettoCache() (*Ristretto, error) {
	var err error
	onceCache.Do(func() {
		var ristrettoCache *ristretto.Cache
		ristrettoCache, err = ristretto.NewCache(&ristretto.Config{
			NumCounters: 1e8,     // Num keys to track frequency of (100M).
			MaxCost:     2 << 30, // Maximum cost of cache (2GB).
			BufferItems: 64,      // Number of keys per Get buffer.
		})
		cache = &Ristretto{
			cache: ristrettoCache,
		}
	})

	if err != nil {
		return nil, err
	}
	return cache, nil
}

// Get returns item with key "key" from cache and a boolean representing whether the item was found or not
func (r *Ristretto) Get(key string) (interface{}, bool) {
	return r.cache.Get(key)
}

// Set sets data to cache with specific ttl. If ttl == -1, default cache ttl value will be used.
func (r *Ristretto) Set(key string, value interface{}, ttl time.Duration) bool {
	if ttl == -1 {
		ttl = RistrettoCacheTTL
	}
	return r.cache.SetWithTTL(key, value, 1, ttl)
}

// Del deletes item with key "key" from cache
func (r *Ristretto) Del(key string) {
	r.cache.Del(key)
}
