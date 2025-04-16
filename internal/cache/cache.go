package cache

import (
	"sync"
	"time"

	"github.com/dgraph-io/ristretto/v2"
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
	Get(string) ([]byte, bool)
	Set(string, []byte, int64, time.Duration) bool
	Del(string)
	Clear()
}

// Because otherwise we cannot mock cache for tests
type storage struct {
	cache *ristretto.Cache[string, []byte]
}

// NewRistrettoCache initializes a cache
var NewRistrettoCache = func() (*Ristretto, error) {
	var err error
	onceCache.Do(func() {
		var ristrettoCache *ristretto.Cache[string, []byte]
		ristrettoCache, err = ristretto.NewCache(&ristretto.Config[string, []byte]{
			// Maximum number of items in cache
			// A recommended number is expected maximum times 10
			// so 30 * 10 = 300
			NumCounters: 300,
			// Maximum size of cache
			// Maximum chunk size that is requested is 32MiB.
			// 1GiB cache can fit 32 items of size 32MiB each
			// or more items if smaller than 32MiB, as long as
			// there are not more than 300 items.
			// Runtime seems to allocate roughly double the size of max cache size,
			// so this now allocates ~2GiB of memory during runtime.
			MaxCost:     1 << 30, // 1GiB
			BufferItems: 64,
		})

		if err == nil {
			cache = &Ristretto{Cacheable: &storage{ristrettoCache}}
		}
	})

	return cache, err
}

// Get returns item behind key "key" and a boolean representing whether the item was found or not
func (s *storage) Get(key string) ([]byte, bool) {
	return s.cache.Get(key)
}

// Set stores data to cache with specific key and ttl. If ttl == -1, RistrettoCacheTTL will be used.
func (s *storage) Set(key string, value []byte, cost int64, ttl time.Duration) bool {
	if ttl == -1 {
		ttl = RistrettoCacheTTL
	}

	ok := s.cache.SetWithTTL(key, value, cost, ttl)
	if ok {
		s.cache.Wait()
	}

	return ok
}

// Del deletes item with key "key" from cache
func (s *storage) Del(key string) {
	s.cache.Del(key)
}

func (s *storage) Clear() {
	s.cache.Clear()
}
