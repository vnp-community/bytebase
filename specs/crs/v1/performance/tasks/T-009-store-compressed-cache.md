# T-009: Store — CompressedSchemaCache (L2)

| Field | Value |
|-------|-------|
| **Task ID** | T-009 |
| **Solution** | SOL-PERF-002 |
| **Type** | New file |
| **Priority** | P0 |
| **Depends on** | None |
| **Blocks** | T-010 |

## Objective

Tạo `CompressedSchemaCache` — L2 cache lưu gzip-compressed proto bytes cho schema metadata.

## Target File

`backend/store/cache_compressed.go` (new)

## Full Implementation

```go
package store

import (
    "bytes"
    "compress/gzip"
    "sync"
    "time"

    "github.com/hashicorp/golang-lru/v2/expirable"
    "google.golang.org/protobuf/proto"

    storepb "github.com/bytebase/bytebase/backend/generated-go/store"
    "github.com/bytebase/bytebase/backend/store/model"
)

type CompressedSchemaCache struct {
    mu    sync.RWMutex
    cache *expirable.LRU[string, []byte]
}

func NewCompressedSchemaCache(size int, ttl time.Duration) *CompressedSchemaCache {
    return &CompressedSchemaCache{
        cache: expirable.NewLRU[string, []byte](size, nil, ttl),
    }
}

func (c *CompressedSchemaCache) Get(key string) (*model.DatabaseMetadata, bool) {
    c.mu.RLock()
    compressed, ok := c.cache.Get(key)
    c.mu.RUnlock()
    if !ok {
        return nil, false
    }

    reader, err := gzip.NewReader(bytes.NewReader(compressed))
    if err != nil {
        return nil, false
    }
    defer reader.Close()

    var buf bytes.Buffer
    if _, err := buf.ReadFrom(reader); err != nil {
        return nil, false
    }

    metadata := &storepb.DatabaseSchemaMetadata{}
    if err := proto.Unmarshal(buf.Bytes(), metadata); err != nil {
        return nil, false
    }

    return model.NewDatabaseMetadataFromProto(metadata), true
}

func (c *CompressedSchemaCache) Add(key string, metadata *model.DatabaseMetadata) {
    protoBytes, err := proto.Marshal(metadata.GetProto())
    if err != nil {
        return
    }

    var buf bytes.Buffer
    writer := gzip.NewWriter(&buf)
    writer.Write(protoBytes)
    writer.Close()

    c.mu.Lock()
    c.cache.Add(key, buf.Bytes())
    c.mu.Unlock()
}

func (c *CompressedSchemaCache) Remove(key string) {
    c.mu.Lock()
    c.cache.Remove(key)
    c.mu.Unlock()
}
```

## Integration

Add field to `Store` struct in `store.go`:

```go
dbSchemaL2Cache *CompressedSchemaCache
```

Initialize in `New()`:

```go
schemaL2Size := adaptiveCacheSize(dbCount, 5000, 100000, 25)
dbSchemaL2Cache := NewCompressedSchemaCache(schemaL2Size, 30*time.Minute)
```
