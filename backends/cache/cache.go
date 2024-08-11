package cache

import (
	"time"

	gcache "github.com/patrickmn/go-cache"
)

const (
	UserPrefix = "user"
)

type ICache interface {
	Add(key string, value interface{}, exp ...time.Duration) error
	Set(key string, value interface{}, exp ...time.Duration)
	Get(key string) (value interface{}, ok bool)
	Contains(key string) (exists bool)
	Remove(key string) bool
}

type Cache struct {
	*gcache.Cache
}

func New() (*Cache, error) {
	return &Cache{
		Cache: gcache.New(gcache.NoExpiration, time.Minute),
	}, nil
}

// Add will error if adding a key that already exists in cache; accepts an
// optional expiration time.
func (c *Cache) Add(key string, value interface{}, exp ...time.Duration) error {
	if len(exp) > 0 {
		return c.Cache.Add(key, value, exp[0])
	}

	return c.Cache.Add(key, value, gcache.NoExpiration)
}

// Set will add OR overwrite an element in the cache; accepts an optional
// expiration time.
func (c *Cache) Set(key string, value interface{}, exp ...time.Duration) {
	if len(exp) > 0 {
		c.Cache.Set(key, value, exp[0])
		return
	}

	c.Cache.Set(key, value, gcache.NoExpiration)
}

func (c *Cache) Get(key string) (interface{}, bool) {
	return c.Cache.Get(key)
}

func (c *Cache) Contains(key string) bool {
	_, ok := c.Cache.Get(key)
	return ok
}

func (c *Cache) Remove(key string) bool {
	_, ok := c.Cache.Get(key)
	if !ok {
		return false
	}

	c.Cache.Delete(key)

	return true
}
