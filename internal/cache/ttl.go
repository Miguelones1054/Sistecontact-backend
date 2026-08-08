package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value  V
	expiry time.Time
}

// TTL es una caché concurrente con expiración por clave.
type TTL[K comparable, V any] struct {
	mu       sync.RWMutex
	items    map[K]entry[V]
	ttl      time.Duration
	stopOnce sync.Once
	stop     chan struct{}
}

func New[K comparable, V any](ttl, cleanup time.Duration) *TTL[K, V] {
	c := &TTL[K, V]{
		items: make(map[K]entry[V]),
		ttl:   ttl,
		stop:  make(chan struct{}),
	}
	go c.cleanup(cleanup)
	return c
}

func (c *TTL[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		var zero V
		return zero, false
	}
	if time.Now().After(e.expiry) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *TTL[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.items[key] = entry[V]{value: value, expiry: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *TTL[K, V]) Close() {
	c.stopOnce.Do(func() { close(c.stop) })
}

func (c *TTL[K, V]) cleanup(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-t.C:
			now := time.Now()
			c.mu.Lock()
			for k, e := range c.items {
				if now.After(e.expiry) {
					delete(c.items, k)
				}
			}
			c.mu.Unlock()
		}
	}
}
