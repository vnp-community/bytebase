import { authServiceClientConnect } from "@/connect";

const LOCK_NAME = "bb_token_refresh";
const CHANNEL_NAME = "bb_token_refresh";
const LOCK_TIMEOUT_MS = 30_000;
const BROADCAST_WAIT_MS = 10_000;
const MAX_RETRIES = 2;

type RefreshMessage = "complete" | "failed";

let localPromise: Promise<void> | null = null;
let refreshMutex = Promise.resolve();

export async function refreshTokens(): Promise<void> {
  if (typeof navigator.locks?.request !== "function") {
    console.warn("[TokenRefresh] Web Locks unavailable, using fallback");
    refreshMutex = refreshMutex.then(() => authServiceClientConnect.refresh({}));
    await refreshMutex;
    return;
  }
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
      const bc = new BroadcastChannel(CHANNEL_NAME);
      try {
        await authServiceClientConnect.refresh({});
        bc.postMessage("complete" satisfies RefreshMessage);
        return true;
      } catch (error) {
        console.error("[TokenRefresh] Refresh failed:", error);
        bc.postMessage("failed" satisfies RefreshMessage);
        return false;
      } finally {
        bc.close();
      }
    }),
    new Promise<boolean>((_, reject) =>
      setTimeout(() => reject(new Error("Lock timeout")), LOCK_TIMEOUT_MS)
    ),
  ]);
}

function waitForMessage(channel: BroadcastChannel, timeoutMs: number): Promise<boolean> {
  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve(false), timeoutMs);
    channel.onmessage = (event) => { clearTimeout(timer); resolve(event.data === "complete"); };
  });
}
