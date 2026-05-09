package cache

import (
	"context"
	"testing"
	"time"
)

func TestLRUCache_HitMiss(t *testing.T) {
	c, err := NewLRU[string, int](10)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Miss
	_, ok, err := c.Get(ctx, "key1")
	if err != nil || ok {
		t.Errorf("expected miss, got ok=%v err=%v", ok, err)
	}

	// Set + Hit
	if err := c.Set(ctx, "key1", 42, 0); err != nil {
		t.Fatal(err)
	}
	v, ok, err := c.Get(ctx, "key1")
	if err != nil || !ok || v != 42 {
		t.Errorf("expected 42, got v=%v ok=%v err=%v", v, ok, err)
	}

	// Delete + Miss
	if err := c.Delete(ctx, "key1"); err != nil {
		t.Fatal(err)
	}
	_, ok, _ = c.Get(ctx, "key1")
	if ok {
		t.Error("expected miss after delete")
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	c, err := NewLRU[string, int](2)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	c.Set(ctx, "a", 1, 0)
	c.Set(ctx, "b", 2, 0)
	c.Set(ctx, "c", 3, 0) // evicts "a"

	_, ok, _ := c.Get(ctx, "a")
	if ok {
		t.Error("expected eviction of 'a'")
	}
	v, ok, _ := c.Get(ctx, "c")
	if !ok || v != 3 {
		t.Errorf("expected 3, got %v", v)
	}
}

func TestLRUCache_Purge(t *testing.T) {
	c, _ := NewLRU[string, int](10)
	ctx := context.Background()
	c.Set(ctx, "a", 1, 0)
	c.Set(ctx, "b", 2, 0)
	c.Purge(ctx)

	_, ok, _ := c.Get(ctx, "a")
	if ok {
		t.Error("expected miss after purge")
	}
}

func TestNoopCache_AlwaysMisses(t *testing.T) {
	c := NewNoop[string, int]()
	ctx := context.Background()

	c.Set(ctx, "key", 42, time.Minute)
	_, ok, err := c.Get(ctx, "key")
	if err != nil || ok {
		t.Errorf("noop should always miss, got ok=%v err=%v", ok, err)
	}
}

// Compile-time checks.
var _ Cache[string, int] = (*LRUCache[string, int])(nil)
var _ Cache[string, int] = (*NoopCache[string, int])(nil)
