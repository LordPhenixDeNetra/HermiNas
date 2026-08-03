package querybroker

import (
	"testing"
	"time"
)

func TestCacheReturnsSetValueBeforeExpiry(t *testing.T) {
	c := NewCache(time.Minute)
	c.Set("SELECT 1", []byte("1"))

	got, ok := c.Get("SELECT 1")
	if !ok || string(got) != "1" {
		t.Fatalf("expected cache hit with %q, got ok=%v value=%q", "1", ok, got)
	}
}

func TestCacheMissForUnknownKey(t *testing.T) {
	c := NewCache(time.Minute)
	if _, ok := c.Get("SELECT 2"); ok {
		t.Fatal("expected cache miss for a key never set")
	}
}

func TestCacheExpiresAfterTTL(t *testing.T) {
	c := NewCache(10 * time.Millisecond)
	c.Set("SELECT 1", []byte("1"))

	time.Sleep(30 * time.Millisecond)

	if _, ok := c.Get("SELECT 1"); ok {
		t.Fatal("expected cache entry to have expired")
	}
}

func TestZeroTTLDisablesCaching(t *testing.T) {
	c := NewCache(0)
	c.Set("SELECT 1", []byte("1"))
	if _, ok := c.Get("SELECT 1"); ok {
		t.Fatal("zero TTL should mean nothing is ever cached")
	}
}
