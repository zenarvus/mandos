package main

import(
	"container/list"
	"sync"
	"time"
)

var nodeCache = NewLRUCache[string, Node](500)
var attachmentCache = NewTTLCache[string, string](5 * time.Minute)
var queryCache = NewTTLCache[string, []map[string]any](5 * time.Minute)

/////////////////////////////////////// LRU CACHE ///////////////////////////////////////

// Use Golang generics to make it work with any type easier.
type LRUCacheItem[K comparable, V any] struct {
	key   K
	value V
}

type LRUCache[K comparable, V any] struct {
	mu sync.Mutex
	capacity int
	items    map[K]*list.Element
	queue    *list.List
}

// Constructor using Generics
func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	if capacity <= 0 {capacity = 10}
	return &LRUCache[K, V]{
		capacity: capacity,
		items:    make(map[K]*list.Element),
		queue:    list.New(),
	}
}

func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, exists := c.items[key]; exists {
		c.queue.MoveToFront(element)
		// We use type assertion here because list.Element.Value is still 'any'
		return element.Value.(*LRUCacheItem[K, V]).value, true
	}
	// Return a "zero value" if not found
	var zero V
	return zero, false
}

// Put adds or updates an item in the cache
func (c *LRUCache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If the item already exists, update it and move to front
	if element, exists := c.items[key]; exists {
		c.queue.MoveToFront(element)
		// Update the value inside the list element
		element.Value.(*LRUCacheItem[K, V]).value = value
		return
	}

	// If the cache is full, evict the oldest item
	if c.queue.Len() >= c.capacity {
		// Get the element at the back (Least Recently Used)
		oldest := c.queue.Back()
		if oldest != nil {
			// Remove from list
			c.queue.Remove(oldest)
			
			// To remove from map, we need the key. 
			// We cast the Value back to our struct to get the key.
			kv := oldest.Value.(*LRUCacheItem[K, V])
			delete(c.items, kv.key)
		}
	}
	// Add the new item to the front (Most Recently Used)
	newItem := &LRUCacheItem[K, V]{
		key:   key,
		value: value,
	}
	element := c.queue.PushFront(newItem)
	c.items[key] = element
}

// Function to update the element in the cache if it exists, without changing its order.
// Update changes the value of an existing key WITHOUT moving it to the front.
// If the key does not exist, it does nothing and returns false.
func (c *LRUCache[K, V]) Update(key K, value V) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if the item exists in our map
	element, exists := c.items[key]
	if !exists { return false } // Do nothing if it does not exists.

	// Update the value inside the existing list element
	// We do NOT call c.queue.MoveToFront(element) here
	kv := element.Value.(*LRUCacheItem[K, V])
	kv.value = value
	
	return true
}

// Delete removes an item from the cache completely.
// It returns true if the item was found and removed, false otherwise.
func (c *LRUCache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if the key exists in the map
	element, exists := c.items[key]
	if !exists { return false }

	// Remove the element from the doubly linked list
	c.queue.Remove(element)
	// Delete the key from the map
	delete(c.items, key)

	return true
}

/////////////////////////////////////// TTL CACHE ///////////////////////////////////////

type TTLCacheItem[V any] struct {
	value      V
	expiresAt int64 // UnixNano timestamp
}

type TTLCache[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]TTLCacheItem[V]
}

// NewTTLCache creates a cache that cleans itself every cleanupInterval
func NewTTLCache[K comparable, V any](cleanupInterval time.Duration) *TTLCache[K, V] {
	c := &TTLCache[K, V]{
		items: make(map[K]TTLCacheItem[V]),
	}

	// Background goroutine to delete expired items (Garbage Collection)
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		for range ticker.C {
			c.mu.Lock()
			now := time.Now().UnixNano()
			for k, item := range c.items {
				if now > item.expiresAt { delete(c.items, k) }
			}
			c.mu.Unlock()
		}
	}()

	return c
}

func (c *TTLCache[K, V]) Set(key K, value V, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = TTLCacheItem[V]{
		value:      value,
		expiresAt: time.Now().Add(duration).UnixNano(),
	}
}

func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	item, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		var zero V
		return zero, false
	}

	// Check if it expired since the last cleanup cycle
	if time.Now().UnixNano() > item.expiresAt {
		// Optional: delete immediately on access if expired
		return *new(V), false
	}

	return item.value, true
}
