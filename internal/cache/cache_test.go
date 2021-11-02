package cache

import (
	"testing"
	"time"
)

var wait = 10 * time.Millisecond

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

	ok := c.Set(key, content, -1)
	if !ok {
		t.Fatal("Saving value failed")
	}

	time.Sleep(wait)
	val, ok := c.Get(key)
	if !ok {
		t.Fatal("Could not find value from cache")
	}
	if val.(string) != content {
		t.Fatalf("Cache returned incorrect value. Expected %q, got %q", content, val.(string))
	}
}

func TestSetAndGet_Expire(t *testing.T) {
	c, err := NewRistrettoCache()
	if err != nil {
		t.Fatalf("Creating cache failed: %s", err.Error())
	}

	if c == nil {
		t.Fatal("Cache is nil")
	}

	key := "muumi"
	content := "To infinity and beyond"

	ok := c.Set(key, content, 2*time.Second)
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

	c.Set("key", "I am information", -1)
	c.Del("key")

	time.Sleep(wait)
	val, ok := c.Get("key")

	if ok {
		t.Errorf("Item was not deleted from cache")
	} else if val != nil {
		t.Errorf("Cache returned non-nil value for item which should have been deleted")
	}
}
