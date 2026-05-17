/**
 * StorageService — centralised, production-grade localStorage wrapper.
 *
 * Features:
 *  - Namespaced keys  (`bb.<ns>.<key>`)
 *  - Optional AES-GCM encryption via Web Crypto API
 *  - TTL (time-to-live) per entry
 *  - Quota monitoring & auto-eviction on QuotaExceeded
 *  - Graceful degradation when storage is unavailable
 *  - PII-free user-scoped keys via deterministic hash
 */

// ─── Types ───────────────────────────────────────────────────

export interface SaveOptions {
  /** Logical namespace – appears in the full key as `bb.<namespace>.<key>`. */
  namespace?: string;
  /** When true, the value is AES-GCM encrypted before writing. */
  encrypt?: boolean;
  /** Time-to-live in milliseconds. Entry auto-expires after this duration. */
  ttlMs?: number;
}

export interface LoadOptions {
  namespace?: string;
  encrypt?: boolean;
}

interface StorageEnvelope<T = unknown> {
  /** Version marker for future migrations. */
  v: 1;
  /** Stored value (plain or encrypted payload). */
  d: T;
  /** Expiry timestamp (epoch ms). Undefined = never expires. */
  e?: number;
  /** True when `d` is an encrypted payload string. */
  enc?: boolean;
}

interface EncryptedPayload {
  /** Initialisation vector (12 bytes, stored as number[]). */
  iv: number[];
  /** Cipher text (stored as number[]). */
  ct: number[];
}

export interface StorageStats {
  usedBytes: number;
  totalCapacity: number;
  usagePercent: number;
  keyCount: number;
}

// ─── Helpers ─────────────────────────────────────────────────

const KEY_PREFIX = "bb";
const QUOTA_WARNING_THRESHOLD = 0.8; // 80 %
const CRYPTO_KEY_NAME = "bb_storage_key";
const CRYPTO_DB_NAME = "bb_storage";

/**
 * Build a deterministic, PII-free hash for user-scoped keys.
 * Uses djb2 hash of the user email.
 *
 * Example:
 *   userScopedKey("recent-visit", "user@example.com")
 *   → "bb.ui.recent-visit.u7k3m2"
 */
export function userScopedKey(baseKey: string, userEmail: string): string {
  let hash = 0;
  for (let i = 0; i < userEmail.length; i++) {
    hash = ((hash << 5) - hash + userEmail.charCodeAt(i)) | 0;
  }
  return `${baseKey}.u${Math.abs(hash).toString(36)}`;
}

// ─── StorageService ──────────────────────────────────────────

export class StorageService {
  private cryptoKey: CryptoKey | null = null;
  private cryptoKeyReady: Promise<CryptoKey> | null = null;
  private storage: Storage;

  constructor(storage: Storage = localStorage) {
    this.storage = storage;
  }

  // ── Key building ────────────────────────────────────────────

  private fullKey(key: string, namespace?: string): string {
    return namespace
      ? `${KEY_PREFIX}.${namespace}.${key}`
      : `${KEY_PREFIX}.${key}`;
  }

  // ── Crypto ──────────────────────────────────────────────────

  private async getEncryptionKey(): Promise<CryptoKey> {
    if (this.cryptoKey) return this.cryptoKey;
    if (this.cryptoKeyReady) return this.cryptoKeyReady;

    this.cryptoKeyReady = (async () => {
      try {
        const db = await new Promise<IDBDatabase>((resolve, reject) => {
          const req = indexedDB.open(CRYPTO_DB_NAME, 1);
          req.onupgradeneeded = () => req.result.createObjectStore("keys");
          req.onsuccess = () => resolve(req.result);
          req.onerror = () => reject(req.error);
        });

        let key = await new Promise<CryptoKey | undefined>((resolve, reject) => {
          const tx = db.transaction("keys", "readonly");
          const req = tx.objectStore("keys").get(CRYPTO_KEY_NAME);
          req.onsuccess = () => resolve(req.result);
          req.onerror = () => reject(req.error);
        });

        if (!key) {
          key = await crypto.subtle.generateKey(
            { name: "AES-GCM", length: 256 },
            false,
            ["encrypt", "decrypt"]
          );
          await new Promise<void>((resolve, reject) => {
            const tx = db.transaction("keys", "readwrite");
            const req = tx.objectStore("keys").put(key, CRYPTO_KEY_NAME);
            req.onsuccess = () => resolve();
            req.onerror = () => reject(req.error);
          });
        }

        this.cryptoKey = key;
        return key;
      } catch (e) {
        console.warn("[StorageService] Failed to initialise encryption key:", e);
        throw e;
      }
    })();

    return this.cryptoKeyReady;
  }

  private async encrypt(plaintext: string): Promise<string> {
    const key = await this.getEncryptionKey();
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const encoded = new TextEncoder().encode(plaintext);
    const encrypted = await crypto.subtle.encrypt(
      { name: "AES-GCM", iv },
      key,
      encoded
    );
    const payload: EncryptedPayload = {
      iv: Array.from(iv),
      ct: Array.from(new Uint8Array(encrypted)),
    };
    return JSON.stringify(payload);
  }

  private async decrypt(ciphertext: string): Promise<string> {
    const key = await this.getEncryptionKey();
    const { iv, ct }: EncryptedPayload = JSON.parse(ciphertext);
    const decrypted = await crypto.subtle.decrypt(
      { name: "AES-GCM", iv: new Uint8Array(iv) },
      key,
      new Uint8Array(ct)
    );
    return new TextDecoder().decode(decrypted);
  }

  // ── Public API ──────────────────────────────────────────────

  /**
   * Save a value to storage.
   * Handles QuotaExceeded by evicting expired entries and retrying once.
   */
  async save<T>(key: string, value: T, options: SaveOptions = {}): Promise<void> {
    const fk = this.fullKey(key, options.namespace);

    let serialised: string;
    if (options.encrypt) {
      const json = JSON.stringify(value);
      const encPayload = await this.encrypt(json);
      const envelope: StorageEnvelope<string> = {
        v: 1,
        d: encPayload,
        enc: true,
        e: options.ttlMs ? Date.now() + options.ttlMs : undefined,
      };
      serialised = JSON.stringify(envelope);
    } else {
      const envelope: StorageEnvelope<T> = {
        v: 1,
        d: value,
        e: options.ttlMs ? Date.now() + options.ttlMs : undefined,
      };
      serialised = JSON.stringify(envelope);
    }

    try {
      this.storage.setItem(fk, serialised);
    } catch (e) {
      if (this.isQuotaExceeded(e)) {
        console.warn("[StorageService] QuotaExceeded — evicting expired entries and retrying");
        this.evictExpired();
        try {
          this.storage.setItem(fk, serialised);
        } catch (e2) {
          console.warn("[StorageService] QuotaExceeded after eviction — save aborted:", fk, e2);
        }
      } else {
        console.warn("[StorageService] Failed to save:", fk, e);
      }
    }
  }

  /**
   * Load a value from storage.
   * Returns fallback if key is missing, expired, or corrupt.
   */
  async load<T>(key: string, options: LoadOptions = {}, fallback: T): Promise<T> {
    const fk = this.fullKey(key, options.namespace);
    const raw = this.storage.getItem(fk);
    if (raw === null) return fallback;

    try {
      const envelope: StorageEnvelope = JSON.parse(raw);

      // TTL check
      if (envelope.e && Date.now() > envelope.e) {
        this.storage.removeItem(fk);
        return fallback;
      }

      if (envelope.enc) {
        const decrypted = await this.decrypt(envelope.d as string);
        return JSON.parse(decrypted) as T;
      }

      return envelope.d as T;
    } catch (e) {
      console.warn("[StorageService] Corrupt data — removing key:", fk, e);
      this.storage.removeItem(fk);
      return fallback;
    }
  }

  /**
   * Synchronous load for non-encrypted entries (most common path).
   * Falls back gracefully for encrypted or corrupt data.
   */
  loadSync<T>(key: string, namespace: string | undefined, fallback: T): T {
    const fk = this.fullKey(key, namespace);
    const raw = this.storage.getItem(fk);
    if (raw === null) return fallback;

    try {
      const envelope: StorageEnvelope = JSON.parse(raw);
      if (envelope.e && Date.now() > envelope.e) {
        this.storage.removeItem(fk);
        return fallback;
      }
      if (envelope.enc) {
        console.warn("[StorageService] loadSync cannot decrypt — use load() instead");
        return fallback;
      }
      return envelope.d as T;
    } catch (e) {
      console.warn("[StorageService] Corrupt data (sync) — removing key:", fk, e);
      this.storage.removeItem(fk);
      return fallback;
    }
  }

  /**
   * Synchronous save for non-encrypted entries (most common path).
   */
  saveSync<T>(key: string, value: T, options: Omit<SaveOptions, "encrypt"> = {}): void {
    const fk = this.fullKey(key, options.namespace);
    const envelope: StorageEnvelope<T> = {
      v: 1,
      d: value,
      e: options.ttlMs ? Date.now() + options.ttlMs : undefined,
    };

    try {
      this.storage.setItem(fk, JSON.stringify(envelope));
    } catch (e) {
      if (this.isQuotaExceeded(e)) {
        console.warn("[StorageService] QuotaExceeded (sync) — evicting and retrying");
        this.evictExpired();
        try {
          this.storage.setItem(fk, JSON.stringify(envelope));
        } catch (e2) {
          console.warn("[StorageService] QuotaExceeded after eviction (sync):", fk, e2);
        }
      } else {
        console.warn("[StorageService] Failed to save (sync):", fk, e);
      }
    }
  }

  /** Remove a single key. */
  remove(key: string, namespace?: string): void {
    const fk = this.fullKey(key, namespace);
    try {
      this.storage.removeItem(fk);
    } catch (e) {
      console.warn("[StorageService] Failed to remove:", fk, e);
    }
  }

  /** Clear all entries within a namespace. */
  clearNamespace(namespace: string): void {
    const prefix = `${KEY_PREFIX}.${namespace}.`;
    const keysToRemove: string[] = [];
    for (let i = 0; i < this.storage.length; i++) {
      const k = this.storage.key(i);
      if (k?.startsWith(prefix)) {
        keysToRemove.push(k);
      }
    }
    for (const k of keysToRemove) {
      this.storage.removeItem(k);
    }
  }

  /** Sweep all namespaces and remove expired entries. */
  evictExpired(): number {
    let evicted = 0;
    const now = Date.now();
    const keysToRemove: string[] = [];

    for (let i = 0; i < this.storage.length; i++) {
      const k = this.storage.key(i);
      if (!k?.startsWith(KEY_PREFIX)) continue;

      try {
        const raw = this.storage.getItem(k);
        if (!raw) continue;
        const envelope: StorageEnvelope = JSON.parse(raw);
        if (envelope.e && now > envelope.e) {
          keysToRemove.push(k);
        }
      } catch {
        // Corrupt entry — mark for removal
        keysToRemove.push(k!);
      }
    }

    for (const k of keysToRemove) {
      this.storage.removeItem(k);
      evicted++;
    }

    if (evicted > 0) {
      console.debug(`[StorageService] Evicted ${evicted} expired entries`);
    }
    return evicted;
  }

  /** Get storage usage stats. */
  getStats(): StorageStats {
    let usedBytes = 0;
    let keyCount = 0;

    for (let i = 0; i < this.storage.length; i++) {
      const key = this.storage.key(i);
      if (!key) continue;
      const value = this.storage.getItem(key) || "";
      usedBytes += key.length * 2 + value.length * 2; // UTF-16
      keyCount++;
    }

    // Most browsers allow ~5MB for localStorage
    const totalCapacity = 5 * 1024 * 1024;
    return {
      usedBytes,
      totalCapacity,
      usagePercent: usedBytes / totalCapacity,
      keyCount,
    };
  }

  /** Check if storage usage exceeds 80% threshold. */
  isQuotaWarning(): boolean {
    return this.getStats().usagePercent > QUOTA_WARNING_THRESHOLD;
  }

  // ── Internals ───────────────────────────────────────────────

  private isQuotaExceeded(e: unknown): boolean {
    if (e instanceof DOMException) {
      return (
        e.code === DOMException.QUOTA_EXCEEDED_ERR ||
        e.name === "QuotaExceededError"
      );
    }
    return false;
  }
}

/** Singleton instance for application-wide use. */
export const storageService = new StorageService();
