# TASK-L-001: Resilient Lock Pattern

> **Source**: SOL-LIM-003 §2.1 | **Priority**: P1 | **Effort**: 3h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/connect/refreshToken.ts` (toàn bộ logic refresh)

## What
Thay thế refreshToken logic hiện tại bằng Resilient Lock Pattern: eager BroadcastChannel, lock timeout, và retry with max attempts.

## Implementation

### Toàn bộ `refreshToken.ts` — Redesign
```typescript
const LOCK_NAME = "bb_token_refresh";
const CHANNEL_NAME = "bb_token_refresh";
const LOCK_TIMEOUT_MS = 30_000;
const BROADCAST_WAIT_MS = 10_000;
const MAX_RETRIES = 2;

let localPromise: Promise<void> | null = null;

export async function refreshTokens(): Promise<void> {
  if (localPromise) return localPromise;
  localPromise = doRefreshWithRetry().finally(() => { localPromise = null; });
  return localPromise;
}

async function doRefreshWithRetry(attempt = 0): Promise<void> {
  // CRITICAL: Create BroadcastChannel BEFORE lock attempt
  const channel = new BroadcastChannel(CHANNEL_NAME);
  try {
    const acquired = await tryAcquireWithTimeout();
    if (acquired) { channel.close(); return; }
    const received = await waitForMessage(channel, BROADCAST_WAIT_MS);
    if (received) return;
    if (attempt < MAX_RETRIES) return doRefreshWithRetry(attempt + 1);
    throw new Error("Token refresh failed after retries");
  } finally {
    try { channel.close(); } catch {}
  }
}

async function tryAcquireWithTimeout(): Promise<boolean> {
  return Promise.race([
    navigator.locks.request(LOCK_NAME, { ifAvailable: true }, async (lock) => {
      if (!lock) return false;
      await authServiceClientConnect.refresh({});
      const bc = new BroadcastChannel(CHANNEL_NAME);
      bc.postMessage("complete");
      bc.close();
      return true;
    }),
    new Promise<boolean>((_, reject) =>
      setTimeout(() => reject(new Error("Lock timeout")), LOCK_TIMEOUT_MS)
    ),
  ]);
}

function waitForMessage(channel: BroadcastChannel, timeoutMs: number): Promise<boolean> {
  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve(false), timeoutMs);
    channel.onmessage = () => { clearTimeout(timer); resolve(true); };
  });
}
```

## AC
- [ ] BroadcastChannel created BEFORE lock attempt (no message loss window)
- [ ] Lock holder timeout after 30s (no infinite starvation)
- [ ] Max 2 retries, then explicit Error thrown
- [ ] `localPromise` dedup prevents concurrent refresh from same tab
