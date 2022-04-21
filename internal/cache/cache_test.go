package cache

import (
	"testing"
	"time"
)

const wait = 10 * time.Millisecond

func TestNewRistrettoCache(t *testing.T) {
	c, err := NewRistrettoCache()
	if err != nil {
		t.Fatalf("Creating cache failed: %s", err.Error())
	}

	if c == nil {
		t.Fatal("Cache is nil")
	}

	c2, err2 := NewRistrettoCache()
	if err2 != nil {
		t.Fatalf("Second call failed: %s", err.Error())
	}

	if c2 != c {
		t.Fatalf("Second call returned a different cache. Expected address %p, received %p", c, c2)
	}
}

func TestSetAndGet(t *testing.T) {
	c, err := NewRistrettoCache()
	if err != nil {
		t.Fatalf("Creating cache failed: %s", err.Error())
	}
	if c == nil {
		t.Fatal("Cache is nil")
	}

	key := "bob"
	content := "¿why is a raven like a writing desk?"

	ok := c.Set(key, content, int64(len(content)), -1)
	if !ok {
		t.Fatal("Saving value failed")
	}

	time.Sleep(wait)
	val, ok := c.Get(key)
	if !ok {
		t.Fatal("Could not find value from cache")
	}
	if str, ok := val.(string); !ok {
		t.Fatalf("Stored value is not a string")
	} else if str != content {
		t.Fatalf("Cache returned incorrect value\nExpected=%s\nReceived=%s", content, str)
	}
}

func TestSetAndGet_Expired(t *testing.T) {
	c, err := NewRistrettoCache()
	if err != nil {
		t.Fatalf("Creating cache failed: %s", err.Error())
	}
	if c == nil {
		t.Fatal("Cache is nil")
	}

	key := "muumi"
	content := "To infinity and beyond"

	ok := c.Set(key, content, int64(len(content)), 2*time.Second)
	if !ok {
		t.Fatal("Saving value failed")
	}

	time.Sleep(3 * time.Second)
	_, ok = c.Get(key)
	if ok {
		t.Fatal("Cache returned value even though item should have expired")
	}
}

func TestDel(t *testing.T) {
	c, err := NewRistrettoCache()
	if err != nil {
		t.Fatalf("Creating cache failed: %s", err.Error())
	}
	if c == nil {
		t.Fatal("Cache is nil")
	}

	c.Set("key", "I am information", int64(len("I am information")), -1)
	c.Del("key")

	time.Sleep(wait)
	_, ok := c.Get("key")

	if ok {
		t.Fatalf("Item was not deleted from cache")
	}
}

func TestClear(t *testing.T) {
	c, err := NewRistrettoCache()
	if err != nil {
		t.Fatalf("Creating cache failed: %s", err.Error())
	}
	if c == nil {
		t.Fatal("Cache is nil")
	}

	c.Set("key", "I am information", int64(len("I am information")), -1)
	c.Set("key2", "More information", int64(len("More information")), -1)
	c.Set("key3", "very secret info", int64(len("very secret info")), -1)

	time.Sleep(wait)
	c.Clear()

	if value, ok := c.Get("key"); ok {
		t.Errorf("Key 'key' with value %q was not cleared from cache", value)
	}
	if value, ok := c.Get("key2"); ok {
		t.Errorf("Key 'key2' with value %q was not cleared from cache", value)
	}
	if value, ok := c.Get("key3"); ok {
		t.Errorf("Key 'key3' with value %q was not cleared from cache", value)
	}
}
