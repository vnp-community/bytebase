# Solution: CR-PERF-007 — Frontend Performance

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-PERF-007                              |
| **Solution ID**| SOL-PERF-007                             |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-08                               |
| **Arch Refs**  | L1 (Presentation), L2 (Gateway)          |
| **TDD Refs**   | §10 Frontend Architecture, §10.2 API Client |

---

## 1. Solution Overview

Từ Architecture L1 và TDD §10.1, frontend là hybrid Vue 3 + React 19. Database listing và schema diagram là 2 bottleneck chính ở scale 200K.

Giải pháp tập trung vào **React 19 layer** (new code):
1. **Virtual scrolling** via `@tanstack/react-virtual`
2. **Cursor pagination** tận dụng SOL-PERF-005 API
3. **BASIC view** cho list — chỉ load metadata on demand
4. **Web Worker** cho schema diagram layout

---

## 2. Detailed Technical Design

### 2.1 Virtual Scrolling for Database List

**File**: `frontend/src/react/components/DatabaseList/VirtualDatabaseList.tsx` (new)

```tsx
import { useVirtualizer } from '@tanstack/react-virtual';
import { useInfiniteQuery } from '@tanstack/react-query';
import { useRef, useCallback } from 'react';

interface VirtualDatabaseListProps {
  workspace: string;
  projectFilter?: string;
}

export function VirtualDatabaseList({ workspace, projectFilter }: VirtualDatabaseListProps) {
  const parentRef = useRef<HTMLDivElement>(null);

  // Infinite query with cursor pagination
  const {
    data,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteQuery({
    queryKey: ['databases', workspace, projectFilter],
    queryFn: async ({ pageParam }) => {
      const response = await databaseServiceClient.listDatabases({
        parent: `workspaces/${workspace}`,
        filter: projectFilter ? `project = "${projectFilter}"` : '',
        pageSize: 100,
        pageCursor: pageParam || '',
        view: DatabaseView.DATABASE_VIEW_BASIC,  // Skip metadata
      });
      return response;
    },
    getNextPageParam: (lastPage) => lastPage.nextPageCursor || undefined,
    initialPageParam: '',
  });

  // Flatten all pages into single array
  const allDatabases = data?.pages.flatMap(p => p.databases) ?? [];

  const virtualizer = useVirtualizer({
    count: allDatabases.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 48,  // Row height in px
    overscan: 20,            // Render 20 extra rows above/below viewport
  });

  // Fetch next page when scrolled near bottom
  const handleScroll = useCallback(() => {
    const el = parentRef.current;
    if (!el) return;
    const scrollBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    if (scrollBottom < 500 && hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [fetchNextPage, hasNextPage, isFetchingNextPage]);

  return (
    <div
      ref={parentRef}
      onScroll={handleScroll}
      style={{ height: '100%', overflow: 'auto' }}
    >
      <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
        {virtualizer.getVirtualItems().map(virtualRow => {
          const db = allDatabases[virtualRow.index];
          return (
            <div
              key={virtualRow.key}
              style={{
                position: 'absolute',
                top: virtualRow.start,
                height: virtualRow.size,
                width: '100%',
              }}
            >
              <DatabaseListRow database={db} />
            </div>
          );
        })}
      </div>
    </div>
  );
}
```

### 2.2 Server-Side Search Integration

Từ TDD §10.2, API client sử dụng ConnectRPC generated client.

**File**: `frontend/src/react/hooks/useDatabaseSearch.ts` (new)

```typescript
import { useState, useCallback } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useDebouncedValue } from './useDebouncedValue';

export function useDatabaseSearch(workspace: string) {
  const [searchTerm, setSearchTerm] = useState('');
  const debouncedSearch = useDebouncedValue(searchTerm, 200);

  const { data: searchResults, isLoading } = useQuery({
    queryKey: ['database-search', workspace, debouncedSearch],
    queryFn: async () => {
      if (!debouncedSearch || debouncedSearch.length < 2) return null;

      const response = await databaseServiceClient.listDatabases({
        parent: `workspaces/${workspace}`,
        filter: `name.contains("${debouncedSearch}")`,
        pageSize: 50,
        view: DatabaseView.DATABASE_VIEW_BASIC,
      });
      return response.databases;
    },
    enabled: debouncedSearch.length >= 2,
    staleTime: 30_000,
  });

  return {
    searchTerm,
    setSearchTerm,
    searchResults,
    isSearching: isLoading,
  };
}
```

### 2.3 Lazy Schema Diagram via Web Worker

Schema diagram (Architecture L1: ELK.js, D3-shape) crashes with >500 tables. Move layout computation to Web Worker.

**File**: `frontend/src/react/workers/schemaLayout.worker.ts` (new)

```typescript
// Web Worker for ELK.js layout computation
import ELK from 'elkjs/lib/elk.bundled';

const elk = new ELK();

self.onmessage = async (event: MessageEvent) => {
  const { tables, relationships, viewport } = event.data;

  // Filter to viewport-visible tables first
  const visibleTables = filterToViewport(tables, viewport);
  const visibleRelationships = relationships.filter(
    r => visibleTables.has(r.source) || visibleTables.has(r.target)
  );

  // ELK layout computation (CPU intensive)
  const graph = {
    id: 'root',
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': 'RIGHT',
      'elk.spacing.nodeNode': '50',
    },
    children: [...visibleTables.values()].map(t => ({
      id: t.name,
      width: 200,
      height: 40 + t.columns.length * 20,
    })),
    edges: visibleRelationships.map(r => ({
      id: `${r.source}-${r.target}`,
      sources: [r.source],
      targets: [r.target],
    })),
  };

  const layout = await elk.layout(graph);
  self.postMessage({ layout, totalTables: tables.length });
};
```

**File**: `frontend/src/react/hooks/useSchemaLayout.ts` (new)

```typescript
import { useEffect, useState, useRef } from 'react';

export function useSchemaLayout(tables: Table[], relationships: Relationship[]) {
  const workerRef = useRef<Worker | null>(null);
  const [layout, setLayout] = useState(null);

  useEffect(() => {
    workerRef.current = new Worker(
      new URL('../workers/schemaLayout.worker.ts', import.meta.url),
      { type: 'module' }
    );

    workerRef.current.onmessage = (event) => {
      setLayout(event.data.layout);
    };

    return () => workerRef.current?.terminate();
  }, []);

  useEffect(() => {
    workerRef.current?.postMessage({
      tables,
      relationships,
      viewport: { x: 0, y: 0, width: window.innerWidth, height: window.innerHeight },
    });
  }, [tables, relationships]);

  return layout;
}
```

### 2.4 Paginated State Management

Từ TDD §10.1, React layer sử dụng Zustand. Replace full dataset store:

**File**: `frontend/src/react/stores/databaseStore.ts` (modify)

```typescript
import { create } from 'zustand';

interface DatabaseState {
  // Windowed: only current page + prefetch
  currentPage: Database[];
  prefetchedPage: Database[];
  totalCount: number;
  cursor: string;
  hasMore: boolean;

  // Actions
  setCurrentPage: (databases: Database[], cursor: string, hasMore: boolean) => void;
  setPrefetchedPage: (databases: Database[]) => void;
  setTotalCount: (count: number) => void;
}

export const useDatabaseStore = create<DatabaseState>((set) => ({
  currentPage: [],
  prefetchedPage: [],
  totalCount: 0,
  cursor: '',
  hasMore: true,

  setCurrentPage: (databases, cursor, hasMore) =>
    set({ currentPage: databases, cursor, hasMore }),
  setPrefetchedPage: (databases) =>
    set({ prefetchedPage: databases }),
  setTotalCount: (count) =>
    set({ totalCount: count }),
}));
```

### 2.5 Vue Bridge — Database Count Display

Từ Architecture L1, Bridge layer (`useVueState`) kết nối React với Vue/Pinia state.

```typescript
// In Vue/Pinia — expose total count for header display
// React component reads via useVueState()
import { useVueState } from '@/hooks/useVueState';

function DatabaseHeader() {
  const totalCount = useVueState(() => useDatabaseV1Store().totalDatabaseCount);
  return <h2>Databases ({totalCount.toLocaleString()})</h2>;
}
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| L1 (Frontend) | **HIGH** | Virtual scrolling, Web Worker, paginated state |
| L2 (Gateway) | **NONE** | No server changes needed |
| L4 (Service) | **NONE** | Uses BASIC view from SOL-PERF-005 |

---

## 4. Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `@tanstack/react-virtual` | ^3.x | Virtual scrolling |
| `@tanstack/react-query` | ^5.x | Data fetching + infinite scroll |
| `elkjs` | ^0.9.x | Already in use — move to Worker |

---

## 5. Performance Estimates

| Metric | Before (10K) | After (200K) |
|--------|-------------|--------------|
| Initial load time | ~5s | ≤ 1s |
| DOM nodes rendered | 10,000 | ~50 |
| Scroll FPS | ~15fps | ≥ 60fps |
| Search latency | ~1s (client filter) | ~200ms (server) |
| Schema 1000 tables | crash | ~2s (Worker) |
| Tab memory (200K) | ~300MB+ | ≤ 40MB |
